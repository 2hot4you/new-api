"""Behavioral backup/restore contracts using a Docker boundary double."""
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "deploy/backup-postgres.sh"
CONTAINER_ID = "b" * 64
PRIVATE_DIAGNOSTIC = "PRIVATE restore SQL row and credential details"

DOCKER_DOUBLE = r'''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
args = sys.argv[1:]
with open(os.environ["MOCK_CALLS"], "a") as log:
    log.write(json.dumps(["docker"] + args) + "\n")
state_path = pathlib.Path(os.environ["MOCK_STATE"])
def option(name):
    return args[args.index(name) + 1]
if args[0] == "inspect":
    if option("--format") == "{{.Config.Image}}":
        print("fixture/previous-image:immutable")
    elif "NanoCpus" in option("--format"):
        print("0 0 0" if os.environ.get("MOCK_UNBOUNDED") == "1" else "1073741824 1073741824 1000000000")
    else:
        print("rw,nosuid,nodev,size=768m")
elif args[0] == "run":
    env = dict(os.environ)
    for line in pathlib.Path(option("--env-file")).read_text().splitlines():
        if line and not line.startswith("#"):
            key, value = line.split("=", 1)
            env[key] = value
    for index, value in enumerate(args[:-1]):
        if value == "--env":
            key, value = args[index + 1].split("=", 1)
            env[key] = value
    mount = option("--volume").split(":")[0]
    if os.environ.get("MOCK_LATE_FILE") == "1":
        pathlib.Path(mount, env["BACKUP_NAME"]).write_bytes(b"late existing backup")
    command = args[args.index("sh"):]
    command[-1] = command[-1].replace("/backup/", mount + "/")
    raise SystemExit(subprocess.run(command, env=env).returncode)
elif args[0] == "create":
    if os.environ.get("MOCK_CREATE_FAIL") == "1":
        print("--all")
        raise SystemExit(1)
    state_path.write_text(json.dumps({"mount": option("--volume").split(":")[0]}))
    print("b" * 64)
elif args[0] == "start":
    if os.environ.get("MOCK_START_FAIL") == "1":
        print("PRIVATE restore SQL row and credential details", file=sys.stderr)
        raise SystemExit(1)
    print(args[-1])
elif args[0] == "exec":
    state = json.loads(state_path.read_text())
    command = [part.replace("/backup/", state["mount"] + "/") for part in args[2:]]
    if command[0] == "sh" and "/proc/1/comm" in command[-1]:
        state["readiness_polls"] = state.get("readiness_polls", 0) + 1
        state_path.write_text(json.dumps(state))
        pidfile = state_path.with_suffix(".pid1")
        initializing = os.environ.get("MOCK_INITIALIZING") == "1" and state["readiness_polls"] == 1
        pidfile.write_text("docker-entrypoi\n" if initializing else "postgres\n")
        command[-1] = command[-1].replace("/proc/1/comm", str(pidfile))
    if command[0] == "createdb" and os.environ.get("MOCK_INITIALIZING") == "1" and state.get("readiness_polls", 0) < 2:
        print("temporary initialization server stopped", file=sys.stderr)
        raise SystemExit(1)
    raise SystemExit(subprocess.run(command).returncode)
elif args[0] == "rm":
    print(args[-1])
else:
    raise SystemExit("unexpected Docker operation")
'''

POSTGRES_DOUBLE = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
tool = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
with open(os.environ["MOCK_CALLS"], "a") as log:
    log.write(json.dumps([tool] + args) + "\n")
if tool == "pg_dump":
    if "--file" in args:
        pathlib.Path(args[args.index("--file") + 1]).write_bytes(b"fixture custom dump")
    else:
        sys.stdout.buffer.write(b"fixture custom dump")
elif tool == "pg_restore" and "--list" not in args:
    if os.environ.get("MOCK_RESTORE_FAIL") == "1":
        print("PRIVATE restore SQL row and credential details", file=sys.stderr)
        raise SystemExit(1)
elif tool not in ("pg_restore", "createdb", "pg_isready"):
    raise SystemExit("unexpected tool")
'''


class PostgresBackupTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.deploy = self.directory / "deployment"
        self.deploy.mkdir()
        self.backups = self.deploy / "backups"
        self.bin = self.directory / "bin"
        self.bin.mkdir()
        for name, body in {"docker": DOCKER_DOUBLE, **{
            name: POSTGRES_DOUBLE for name in ("pg_dump", "pg_restore", "createdb", "pg_isready")
        }}.items():
            path = self.bin / name
            path.write_text(body)
            path.chmod(0o700)
        self.calls_path = self.directory / "calls.jsonl"
        self.environment = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ["PATH"],
                                MOCK_CALLS=str(self.calls_path), MOCK_STATE=str(self.directory / "state.json"))
        self.write_runtime()

    def write_runtime(self, log_dsn=""):
        self.main_dsn = "postgresql://fixture:private-main@127.0.0.1:5432/main?sslmode=disable"
        (self.deploy / ".env.runtime").write_text("SQL_DSN=" + self.main_dsn + "\nLOG_SQL_DSN=" + log_dsn + "\n")

    def run_backup(self, **environment):
        return subprocess.run(["sh", str(HELPER), str(self.deploy), "fixture.dump"],
                              env=dict(self.environment, **environment), capture_output=True, text=True)

    def calls(self, tool=None):
        rows = [json.loads(line) for line in self.calls_path.read_text().splitlines()]
        return [row for row in rows if not tool or row[0] == tool]

    def assert_private(self):
        self.assertEqual(stat.S_IMODE(self.backups.stat().st_mode), 0o700)
        self.assertEqual(list(self.backups.glob(".*.lock")), [])
        for path in self.backups.iterdir():
            if path.is_file():
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600, path.name)

    def test_success_restores_dump_offline_and_cleans_exact_owned_container(self):
        result = self.run_backup()
        self.assertEqual(result.returncode, 0, result.stderr)
        restores = [row for row in self.calls("pg_restore") if "--list" not in row]
        self.assertEqual(len(restores), 1)
        self.assertIn("--exit-on-error", restores[0])
        created = next(row for row in self.calls("docker") if row[1] == "create")
        self.assertEqual(created[created.index("--network") + 1], "none")
        self.assertIn("--tmpfs", created)
        for option, value in (("--memory", "1g"), ("--memory-swap", "1g"), ("--cpus", "1")):
            self.assertEqual(created[created.index(option) + 1], value)
        self.assertEqual(created[created.index("--tmpfs") + 1], "/var/lib/postgresql/data:rw,nosuid,nodev,size=768m")
        self.assertNotIn("--publish", created)
        self.assertNotIn("-p", created)
        self.assertTrue(created[created.index("--volume") + 1].endswith(":/backup:ro"))
        removed = [row for row in self.calls("docker") if row[1] == "rm"]
        self.assertEqual(len(removed), 1)
        self.assertEqual(removed[0][-1], CONTAINER_ID)
        self.assertTrue((self.backups / "fixture.dump").is_file())
        self.assert_private()

    def test_restore_failure_retains_dump_private_diagnostics_and_cleans_container(self):
        result = self.run_backup(MOCK_RESTORE_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertTrue((self.backups / "fixture.dump").is_file())
        self.assertNotIn(PRIVATE_DIAGNOSTIC, result.stdout + result.stderr)
        private_logs = list(self.backups.glob("*.log"))
        self.assertEqual(len(private_logs), 1)
        self.assertIn(PRIVATE_DIAGNOSTIC, private_logs[0].read_text())
        self.assertEqual([row[-1] for row in self.calls("docker") if row[1] == "rm"], [CONTAINER_ID])
        self.assert_private()

    def test_preexisting_dump_is_never_overwritten(self):
        self.backups.mkdir(mode=0o700)
        original = self.backups / "fixture.dump"
        original.write_bytes(b"previous backup must survive")
        result = self.run_backup()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(original.read_bytes(), b"previous backup must survive")
        self.assertFalse(self.calls_path.exists(), "preexisting backup must fail before Docker")

    def test_separate_log_database_is_dumped_and_restored_separately(self):
        log_dsn = "postgresql://fixture:private-log@127.0.0.1:5432/logs?sslmode=disable"
        self.write_runtime(log_dsn)
        result = self.run_backup()
        self.assertEqual(result.returncode, 0, result.stderr)
        dumps = self.calls("pg_dump")
        self.assertEqual(len(dumps), 2)
        self.assertIn(self.main_dsn, dumps[0])
        self.assertIn(log_dsn, dumps[1])
        restores = [row for row in self.calls("pg_restore") if "--list" not in row]
        self.assertEqual(len(restores), 2)
        databases = [row[row.index("--dbname") + 1] for row in restores]
        self.assertEqual(len(set(databases)), 2)
        self.assertNotIn("private-main", result.stdout + result.stderr)
        self.assertNotIn("private-log", result.stdout + result.stderr)
        self.assert_private()

    def test_preexisting_log_dump_is_never_overwritten(self):
        self.backups.mkdir(mode=0o700)
        original = self.backups / "fixture.dump.log.dump"
        original.write_bytes(b"previous log backup must survive")
        result = self.run_backup()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(original.read_bytes(), b"previous log backup must survive")
        self.assertFalse(self.calls_path.exists())

    def test_dump_noclobber_preserves_a_file_created_after_precheck(self):
        result = self.run_backup(MOCK_LATE_FILE="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.backups / "fixture.dump").read_bytes(), b"late existing backup")
        self.assertEqual(self.calls("pg_dump"), [])

    def test_start_failure_still_cleans_the_exact_created_container(self):
        result = self.run_backup(MOCK_START_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn(PRIVATE_DIAGNOSTIC, result.stdout + result.stderr)
        self.assertEqual([row[-1] for row in self.calls("docker") if row[1] == "rm"], [CONTAINER_ID])
        self.assertTrue((self.backups / "fixture.dump").is_file())
        self.assert_private()

    def test_failed_create_output_is_never_used_as_a_cleanup_target(self):
        result = self.run_backup(MOCK_CREATE_FAIL="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([row for row in self.calls("docker") if row[1] == "rm"], [])
        self.assertTrue((self.backups / "fixture.dump").is_file())

    def test_discarded_resource_limits_fail_before_start_and_retain_dumps(self):
        result = self.run_backup(MOCK_UNBOUNDED="1")
        self.assertNotEqual(result.returncode, 0)
        calls = self.calls("docker")
        self.assertEqual([row for row in calls if row[1] == "start"], [])
        self.assertEqual([row[-1] for row in calls if row[1] == "rm"], [CONTAINER_ID])
        self.assertTrue((self.backups / "fixture.dump").is_file())
        self.assert_private()

    def test_readiness_waits_for_final_postgres_not_temporary_init_server(self):
        result = self.run_backup(MOCK_INITIALIZING="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls("docker")
        first_create_database = next(index for index, row in enumerate(calls) if row[1] == "exec" and row[3] == "createdb")
        probes = [row for row in calls[:first_create_database] if row[1] == "exec" and "/proc/1/comm" in row[-1]]
        self.assertEqual(len(probes), 2)

    def test_identical_log_dsn_only_creates_one_dump(self):
        self.write_runtime(self.main_dsn)
        result = self.run_backup()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(self.calls("pg_dump")), 1)
        self.assertEqual(len([row for row in self.calls("pg_restore") if "--list" not in row]), 1)


if __name__ == "__main__":
    unittest.main()
