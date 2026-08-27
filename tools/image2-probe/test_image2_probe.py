import json
import subprocess
import unittest
from unittest import mock

import image2_probe


class RequestBuildingTests(unittest.TestCase):
    def test_generation_request_preserves_every_selected_parameter(self):
        body = image2_probe.build_request_body(
            operation="generation",
            model="gpt-image-2-2k",
            prompt="city at night",
            size="2048x2048",
            quality="low",
            background="transparent",
            count=2,
            response_format="url",
        )

        self.assertEqual(
            body,
            {
                "async": True,
                "background": "transparent",
                "model": "gpt-image-2-2k",
                "n": 2,
                "prompt": "city at night",
                "quality": "low",
                "response_format": "url",
                "size": "2048x2048",
            },
        )

    def test_edit_request_includes_images_and_optional_mask(self):
        body = image2_probe.build_request_body(
            operation="edit",
            model="gpt-image-2-1k",
            prompt="replace the sky",
            size="1:1",
            quality="medium",
            background="opaque",
            count=1,
            response_format="url",
            images=["https://cdn.example.com/reference.png"],
            mask="https://cdn.example.com/mask.png",
        )

        self.assertEqual(body["images"], ["https://cdn.example.com/reference.png"])
        self.assertEqual(body["mask"], "https://cdn.example.com/mask.png")
        self.assertTrue(body["async"])


class ProviderResponseTests(unittest.TestCase):
    def test_extracts_task_id_and_status_from_common_response_shapes(self):
        self.assertEqual(
            image2_probe.extract_task_id({"id": "task_top", "status": "queued"}),
            "task_top",
        )
        self.assertEqual(
            image2_probe.extract_task_id({"data": {"task_id": "task_nested"}}),
            "task_nested",
        )
        self.assertEqual(
            image2_probe.extract_status({"data": {"status": "completed"}}),
            "completed",
        )

    def test_classifies_terminal_statuses_without_guessing_unknown_values(self):
        self.assertEqual(image2_probe.classify_status("completed"), "success")
        self.assertEqual(image2_probe.classify_status("FAILED"), "failure")
        self.assertEqual(image2_probe.classify_status("processing"), "pending")
        self.assertEqual(image2_probe.classify_status("provider-specific"), "unknown")

    def test_only_transient_poll_http_statuses_are_retried(self):
        for status in (None, 404, 408, 409, 425, 429, 500, 503):
            self.assertTrue(image2_probe.is_retryable_poll_status(status))
        for status in (200, 400, 401, 403, 422):
            self.assertFalse(image2_probe.is_retryable_poll_status(status))

    def test_extracts_all_unique_result_urls_from_completed_response(self):
        payload = {
            "status": "completed",
            "data": [
                {"url": "https://files.example.com/one.png?token=first"},
                {"url": "https://files.example.com/two.png?token=second"},
                {"url": "https://files.example.com/one.png?token=first"},
            ],
            "metadata": {"result_url": "https://files.example.com/three.webp"},
            "unrelated": "not-a-url",
        }

        self.assertEqual(
            image2_probe.extract_result_urls(payload),
            [
                "https://files.example.com/one.png?token=first",
                "https://files.example.com/two.png?token=second",
                "https://files.example.com/three.webp",
            ],
        )


class ValidationTests(unittest.TestCase):
    def test_validates_exact_dimensions_against_model_pixel_limit(self):
        self.assertEqual(
            image2_probe.validate_size("gpt-image-2-4k", "2880*2880"),
            "2880x2880",
        )
        with self.assertRaisesRegex(ValueError, "像素总数"):
            image2_probe.validate_size("gpt-image-2-1k", "2048x2048")
        with self.assertRaisesRegex(ValueError, "16"):
            image2_probe.validate_size("gpt-image-2-2k", "1025x1024")

    def test_accepts_documented_ratio_values(self):
        self.assertEqual(image2_probe.validate_size("gpt-image-2-1k", "16:9"), "16:9")


class PromptInputTests(unittest.TestCase):
    def test_clipboard_prompt_preserves_long_multiline_text_without_truncation(self):
        prompt = "第一行\n" + ("长文本" * 5000) + "\n最后一行"
        completed = subprocess.CompletedProcess(
            args=["/usr/bin/pbpaste"],
            returncode=0,
            stdout=prompt,
            stderr="",
        )

        with mock.patch.object(image2_probe.subprocess, "run", return_value=completed):
            result = image2_probe.read_clipboard_prompt("/usr/bin/pbpaste")

        self.assertEqual(result, prompt)
        self.assertEqual(len(result), len(prompt))

    def test_clipboard_prompt_rejects_empty_or_whitespace_only_content(self):
        completed = subprocess.CompletedProcess(
            args=["/usr/bin/pbpaste"],
            returncode=0,
            stdout=" \n\t",
            stderr="",
        )

        with mock.patch.object(image2_probe.subprocess, "run", return_value=completed):
            with self.assertRaisesRegex(ValueError, "剪贴板中没有 Prompt"):
                image2_probe.read_clipboard_prompt("/usr/bin/pbpaste")


class ReportSafetyTests(unittest.TestCase):
    def test_shareable_report_redacts_keys_and_signed_url_values(self):
        exchange = image2_probe.Exchange(
            label="创建任务",
            method="POST",
            url="https://ai.cangyuansuanli.cn/v1/images/generations",
            request_headers={
                "Authorization": "Bearer sk-secret",
                "Content-Type": "application/json",
            },
            request_body={"model": "gpt-image-2-1k", "prompt": "hello"},
            status=200,
            response_headers={"X-Request-Id": "req_123"},
            response_body=json.dumps(
                {
                    "id": "task_123",
                    "data": [
                        {
                            "url": (
                                "https://files.example.com/image.png"
                                "?token=secret-token&expires=99"
                            )
                        }
                    ],
                }
            ),
            elapsed_seconds=0.25,
        )

        report = image2_probe.render_exchange(exchange)

        self.assertIn("task_123", report)
        self.assertIn("token=REDACTED", report)
        self.assertIn("expires=99", report)
        self.assertNotIn("sk-secret", report)
        self.assertNotIn("secret-token", report)
        self.assertIn("Bearer REDACTED", report)

    def test_response_location_header_does_not_leak_signed_download_token(self):
        headers = image2_probe.redact_headers(
            {
                "Content-Type": "image/png",
                "Location": "https://cdn.example.com/result.png?signature=secret&expires=99",
            }
        )

        self.assertEqual(
            headers["Location"],
            "https://cdn.example.com/result.png?signature=REDACTED&expires=99",
        )
        self.assertNotIn("secret", image2_probe.render_headers(headers))


class TerminalOutputTests(unittest.TestCase):
    def test_terminal_records_include_original_clickable_result_url(self):
        result_url = "https://files.example.com/result.png?token=temporary-secret"
        run = image2_probe.ProbeRun(
            sequence=1,
            operation="generation",
            model="gpt-image-2-1k",
            request_body={"model": "gpt-image-2-1k", "prompt": "hello"},
            started_at="2026-08-27T00:00:00+08:00",
            task_id="task_123",
            final_status="completed",
            result_urls=[result_url],
        )

        output = image2_probe.render_terminal_records([run])

        self.assertIn("结果 URL（原样）", output)
        self.assertIn(result_url, output)
        self.assertNotIn("image2-probe-report.md", output)


if __name__ == "__main__":
    unittest.main()
