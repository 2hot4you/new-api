"""Exercise the SSH forced command without invoking sudo on the test host."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest


ENTRYPOINT = pathlib.Path(__file__).resolve().parents[1] / "model-claudeye-tencent-ssh"
OLD = "ghcr.io/2hot4you/new-api@sha256:" + "a" * 64
NEW = "ghcr.io/2hot4you/new-api@sha256:" + "b" * 64


class ForcedCommandTest(unittest.TestCase):
    def invoke(self, command=None, *extra):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory)
            sudo = path / "sudo-fixture"
            sudo.write_text(
                f"#!{sys.executable}\nimport json, sys\nprint(json.dumps(sys.argv[1:]))\n",
                encoding="utf-8",
            )
            sudo.chmod(0o700)
            wrapper = path / "forced-command"
            original = ENTRYPOINT.read_text(encoding="utf-8") if ENTRYPOINT.exists() else ""
            wrapper.write_text(original.replace("/usr/bin/sudo", str(sudo)), encoding="utf-8")
            env = dict(os.environ)
            env.pop("SSH_ORIGINAL_COMMAND", None)
            if command is not None:
                env["SSH_ORIGINAL_COMMAND"] = command
            return subprocess.run(
                ["/bin/sh", str(wrapper), *extra], env=env, capture_output=True, text=True,
                check=False,
            )

    def test_preflight_only_invokes_fixed_sudo_program(self):
        result = self.invoke("preflight")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout, "expected the fixed sudo command to be invoked")
        self.assertEqual(
            json.loads(result.stdout),
            ["-n", "--", "/usr/local/sbin/model-claudeye-tencent.py", "preflight"],
        )

    def test_release_is_forwarded_as_one_literal_argument(self):
        command = f"release {OLD} {NEW} {'c' * 40}"
        result = self.invoke(command)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(result.stdout, "expected the fixed sudo command to be invoked")
        self.assertEqual(
            json.loads(result.stdout),
            ["-n", "--", "/usr/local/sbin/model-claudeye-tencent.py", command],
        )

    def test_interactive_scp_options_and_shell_syntax_are_rejected(self):
        for command in (None, "", "bash", "scp -t .", "-V", "release x;id", "preflight\nid",
                        "release $(id)", "release x\ty", "release " + "x" * 300,
                        "release x", f"release {OLD} {NEW} {'c' * 39}",
                        f"release {OLD} {NEW.replace('2hot4you', 'foreign')} {'c' * 40}"):
            with self.subTest(command=command):
                result = self.invoke(command)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, "")

    def test_unexpected_positional_arguments_are_rejected(self):
        result = self.invoke("preflight", "unexpected")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")


if __name__ == "__main__":
    unittest.main()
