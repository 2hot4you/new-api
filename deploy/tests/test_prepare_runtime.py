import importlib.util
from pathlib import Path
import os
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('runtime', Path(__file__).parents[1] / 'infra/prepare-runtime.py')
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
BODY = ('SQL_DSN=postgresql://new_api:abc@postgres:5432/new_api?sslmode=disable\n'
        'REDIS_CONN_STRING=redis://:abc@redis:6379/0\n'
        'SESSION_SECRET=' + 'a' * 64 + '\nCRYPTO_SECRET=' + 'b' * 64 + '\n')

class RuntimeTest(unittest.TestCase):
    def test_preserves_secrets_and_site_isolation(self):
        result = m.runtime_text(BODY, 'production-model-claudeye')
        self.assertTrue(result.startswith(BODY))
        self.assertIn('SESSION_COOKIE_TRUSTED_URL=https://model.claudeye.com\n', result)
        self.assertNotIn('HTTP_PROXY', result)
        self.assertNotIn('https://claudeye.com\n', result)

    def test_rejects_extra_duplicate_and_malformed_values(self):
        for body in (BODY + 'HTTP_PROXY=http://bad\n', BODY + 'SQL_DSN=x\n', BODY.replace('@redis:', '@foreign:'), BODY.replace('a'*64, 'short')):
            with self.subTest(body='redacted'):
                with self.assertRaises(ValueError):
                    m.runtime_text(body, 'production-claudeye')

    def test_fresh_private_tree_and_refusal_to_overwrite(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / 'production'
            m.write_tree(target, BODY, os.getuid(), os.getgid())
            self.assertEqual((target / '.env.runtime').read_text(), BODY)
            self.assertEqual((target / '.env.runtime').stat().st_mode & 0o777, 0o600)
            self.assertEqual(target.stat().st_mode & 0o777, 0o700)
            self.assertTrue(all((target / name).is_dir() for name in ('data', 'logs', 'certs')))
            with self.assertRaises(FileExistsError):
                m.write_tree(target, 'replacement', os.getuid(), os.getgid())
            self.assertEqual((target / '.env.runtime').read_text(), BODY)

if __name__ == '__main__':
    unittest.main()
