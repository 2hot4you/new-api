#!/usr/bin/env python3
"""Interactive terminal probe for the Cangyuan gpt-image-2 async API."""

import dataclasses
import datetime as dt
import getpass
import json
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple


BASE_URL = "https://ai.cangyuansuanli.cn"
MODELS = {
    "gpt-image-2-1k": {"default_size": "1024x1024", "max_pixels": 1_048_576},
    "gpt-image-2-2k": {"default_size": "2048x2048", "max_pixels": 4_194_304},
    "gpt-image-2-4k": {"default_size": "2880x2880", "max_pixels": 8_294_400},
}
MIN_PIXELS = 655_360
RATIO_SIZES = {"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"}
SUCCESS_STATUSES = {"completed", "succeeded", "success"}
FAILURE_STATUSES = {"failed", "failure", "canceled", "cancelled", "error"}
PENDING_STATUSES = {
    "queued",
    "submitted",
    "pending",
    "processing",
    "in_progress",
    "running",
}
SENSITIVE_HEADER_NAMES = {
    "authorization",
    "cookie",
    "proxy-authorization",
    "set-cookie",
    "x-api-key",
}
SENSITIVE_VALUE_NAMES = {
    "access_token",
    "api_key",
    "apikey",
    "authorization",
    "credential",
    "key",
    "password",
    "secret",
    "sig",
    "signature",
    "token",
    "x-amz-credential",
    "x-amz-security-token",
    "x-amz-signature",
    "x-goog-credential",
    "x-goog-signature",
}
MAX_JSON_RESPONSE_BYTES = 10 * 1024 * 1024


@dataclasses.dataclass
class Exchange:
    label: str
    method: str
    url: str
    request_headers: Dict[str, str]
    request_body: Optional[Dict[str, Any]] = None
    status: Optional[int] = None
    response_headers: Dict[str, str] = dataclasses.field(default_factory=dict)
    response_body: str = ""
    elapsed_seconds: float = 0.0
    effective_url: str = ""
    redirects: List[Dict[str, Any]] = dataclasses.field(default_factory=list)
    error: str = ""


@dataclasses.dataclass
class ProbeRun:
    sequence: int
    operation: str
    model: str
    request_body: Dict[str, Any]
    started_at: str
    task_id: str = ""
    final_status: str = ""
    exchanges: List[Exchange] = dataclasses.field(default_factory=list)
    result_urls: List[str] = dataclasses.field(default_factory=list)


class RecordingRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Record redirect metadata and prevent cross-origin credential leakage."""

    def __init__(self) -> None:
        super().__init__()
        self.redirects: List[Dict[str, Any]] = []

    def redirect_request(
        self,
        req: urllib.request.Request,
        fp: Any,
        code: int,
        msg: str,
        headers: Any,
        newurl: str,
    ) -> Optional[urllib.request.Request]:
        self.redirects.append(
            {
                "status": code,
                "from": req.full_url,
                "to": newurl,
                "headers": dict(headers.items()),
            }
        )
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        if redirected is None:
            return None
        old_origin = urllib.parse.urlsplit(req.full_url)[:2]
        new_origin = urllib.parse.urlsplit(newurl)[:2]
        if old_origin != new_origin:
            redirected.remove_header("Authorization")
            redirected.remove_header("Proxy-Authorization")
            redirected.remove_header("Cookie")
        return redirected


def build_request_body(
    operation: str,
    model: str,
    prompt: str,
    size: str,
    quality: str,
    background: str,
    count: int,
    response_format: str,
    images: Optional[List[str]] = None,
    mask: str = "",
) -> Dict[str, Any]:
    body: Dict[str, Any] = {
        "async": True,
        "background": background,
        "model": model,
        "n": count,
        "prompt": prompt,
        "quality": quality,
        "response_format": response_format,
        "size": size,
    }
    if operation == "edit":
        body["images"] = list(images or [])
        if mask:
            body["mask"] = mask
    return body


def validate_size(model: str, raw_size: str) -> str:
    size = raw_size.strip().lower().replace("×", "x").replace("*", "x")
    if size in RATIO_SIZES:
        return size
    match = re.fullmatch(r"(\d+)\s*x\s*(\d+)", size)
    if not match:
        raise ValueError("尺寸必须是比例（如 1:1）或精确尺寸（如 1024x1024）")
    width, height = int(match.group(1)), int(match.group(2))
    if width % 16 != 0 or height % 16 != 0:
        raise ValueError("精确尺寸的宽和高都必须是 16 的倍数")
    pixels = width * height
    if pixels < MIN_PIXELS or pixels > int(MODELS[model]["max_pixels"]):
        raise ValueError(
            "像素总数必须在 {}–{} 之间".format(
                MIN_PIXELS, int(MODELS[model]["max_pixels"])
            )
        )
    return "{}x{}".format(width, height)


def extract_task_id(payload: Any) -> str:
    if not isinstance(payload, dict):
        return ""
    for key in ("id", "task_id", "taskId"):
        value = payload.get(key)
        if isinstance(value, (str, int)) and str(value).strip():
            return str(value).strip()
    for key in ("data", "result", "task"):
        nested = payload.get(key)
        if isinstance(nested, dict):
            task_id = extract_task_id(nested)
            if task_id:
                return task_id
    return ""


def extract_status(payload: Any) -> str:
    if not isinstance(payload, dict):
        return ""
    for key in ("status", "state", "task_status"):
        value = payload.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    for key in ("data", "result", "task"):
        nested = payload.get(key)
        if isinstance(nested, dict):
            status = extract_status(nested)
            if status:
                return status
    return ""


def extract_result_urls(payload: Any) -> List[str]:
    result_urls: List[str] = []
    seen = set()

    def visit(value: Any, field_name: str = "") -> None:
        if isinstance(value, dict):
            for key, item in value.items():
                visit(item, str(key))
            return
        if isinstance(value, list):
            for item in value:
                visit(item, field_name)
            return
        if field_name.lower() not in {"url", "result_url", "resulturl"}:
            return
        if not isinstance(value, str) or not value.startswith(("http://", "https://")):
            return
        if value not in seen:
            seen.add(value)
            result_urls.append(value)

    visit(payload)
    return result_urls


def classify_status(status: str) -> str:
    normalized = status.strip().lower().replace("-", "_").replace(" ", "_")
    if normalized in SUCCESS_STATUSES:
        return "success"
    if normalized in FAILURE_STATUSES:
        return "failure"
    if normalized in PENDING_STATUSES:
        return "pending"
    return "unknown"


def is_retryable_poll_status(status: Optional[int]) -> bool:
    if status is None or status >= 500:
        return True
    return status in {404, 408, 409, 425, 429}


def redact_url(raw_url: str) -> str:
    try:
        parsed = urllib.parse.urlsplit(raw_url)
    except ValueError:
        return raw_url
    if not parsed.scheme or not parsed.netloc:
        return raw_url
    query = []
    for name, value in urllib.parse.parse_qsl(parsed.query, keep_blank_values=True):
        if name.lower() in SENSITIVE_VALUE_NAMES:
            value = "REDACTED"
        query.append((name, value))
    return urllib.parse.urlunsplit(
        (parsed.scheme, parsed.netloc, parsed.path, urllib.parse.urlencode(query), parsed.fragment)
    )


def redact_value(value: Any, field_name: str = "") -> Any:
    if field_name.lower() in SENSITIVE_VALUE_NAMES:
        return "REDACTED"
    if isinstance(value, dict):
        return {key: redact_value(item, str(key)) for key, item in value.items()}
    if isinstance(value, list):
        return [redact_value(item) for item in value]
    if isinstance(value, str):
        if value.lower().startswith("bearer "):
            return "Bearer REDACTED"
        if value.startswith("http://") or value.startswith("https://"):
            return redact_url(value)
    return value


def redact_headers(headers: Dict[str, str]) -> Dict[str, str]:
    redacted: Dict[str, str] = {}
    for name, value in headers.items():
        lowered_name = name.lower()
        if lowered_name in SENSITIVE_HEADER_NAMES:
            redacted[name] = (
                "Bearer REDACTED" if lowered_name == "authorization" else "REDACTED"
            )
        elif value.startswith("http://") or value.startswith("https://"):
            redacted[name] = redact_url(value)
        else:
            redacted[name] = value
    return redacted


def redact_response_body(body: str) -> str:
    if not body:
        return ""
    try:
        payload = json.loads(body)
    except (ValueError, TypeError):
        redacted = re.sub(
            r"(?i)(authorization\s*[:=]\s*bearer\s+)[^\s,;]+",
            r"\1REDACTED",
            body,
        )
        return redacted
    return json.dumps(redact_value(payload), ensure_ascii=False, indent=2)


def decode_json_body(body: str) -> Any:
    try:
        return json.loads(body)
    except (ValueError, TypeError):
        return None


def request_json(
    label: str,
    method: str,
    path: str,
    api_key: str,
    body: Optional[Dict[str, Any]] = None,
    timeout: float = 120.0,
) -> Tuple[Exchange, Any]:
    url = BASE_URL + path
    headers = {
        "Accept": "application/json",
        "Authorization": "Bearer " + api_key,
    }
    data = None
    if body is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    exchange = Exchange(label, method, url, headers, body)
    handler = RecordingRedirectHandler()
    opener = urllib.request.build_opener(handler)
    request = urllib.request.Request(url, data=data, headers=headers, method=method)
    started = time.monotonic()
    try:
        with opener.open(request, timeout=timeout) as response:
            raw = response.read(MAX_JSON_RESPONSE_BYTES + 1)
            if len(raw) > MAX_JSON_RESPONSE_BYTES:
                raise ValueError("响应体超过 10 MiB 安全上限")
            exchange.status = response.status
            exchange.response_headers = dict(response.headers.items())
            exchange.response_body = raw.decode("utf-8", errors="replace")
            exchange.effective_url = response.geturl()
    except urllib.error.HTTPError as error:
        raw = error.read(MAX_JSON_RESPONSE_BYTES + 1)
        exchange.status = error.code
        exchange.response_headers = dict(error.headers.items())
        exchange.response_body = raw.decode("utf-8", errors="replace")
        exchange.effective_url = error.geturl()
        if len(raw) > MAX_JSON_RESPONSE_BYTES:
            exchange.error = "响应体超过 10 MiB 安全上限"
    except (urllib.error.URLError, TimeoutError, OSError, ValueError) as error:
        exchange.error = str(error)
    finally:
        exchange.elapsed_seconds = time.monotonic() - started
        exchange.redirects = handler.redirects
    return exchange, decode_json_body(exchange.response_body)


def pretty_json(value: Any) -> str:
    return json.dumps(redact_value(value), ensure_ascii=False, indent=2)


def render_headers(headers: Dict[str, str]) -> str:
    safe_headers = redact_headers(headers)
    return "\n".join("{}: {}".format(name, value) for name, value in safe_headers.items())


def render_curl(exchange: Exchange) -> str:
    lines = ["curl --request {} '{}'".format(exchange.method, redact_url(exchange.url))]
    for name, value in redact_headers(exchange.request_headers).items():
        if name.lower() == "authorization":
            value = "Bearer $IMAGE2_API_KEY"
        lines.append("  --header '{}: {}'".format(name, value.replace("'", "'\\''")))
    if exchange.request_body is not None:
        payload = json.dumps(exchange.request_body, ensure_ascii=False, indent=2)
        lines.append("  --data-raw '{}'".format(payload.replace("'", "'\\''")))
    return " \\\n".join(lines)


def render_exchange(exchange: Exchange) -> str:
    sections = [
        "#### {}".format(exchange.label),
        "",
        "- Method: `{}`".format(exchange.method),
        "- URL: `{}`".format(redact_url(exchange.url)),
        "- HTTP status: `{}`".format(exchange.status if exchange.status is not None else "N/A"),
        "- Elapsed: `{:.3f}s`".format(exchange.elapsed_seconds),
    ]
    if exchange.effective_url:
        sections.append("- Effective URL: `{}`".format(redact_url(exchange.effective_url)))
    if exchange.error:
        sections.append("- Error: `{}`".format(exchange.error))
    sections.extend(
        [
            "",
            "Request headers:",
            "",
            "```http",
            render_headers(exchange.request_headers) or "<none>",
            "```",
            "",
            "Request curl:",
            "",
            "```bash",
            render_curl(exchange),
            "```",
        ]
    )
    if exchange.request_body is not None:
        sections.extend(
            ["", "Request body:", "", "```json", pretty_json(exchange.request_body), "```"]
        )
    if exchange.redirects:
        sections.extend(["", "Redirect chain:", ""])
        for index, redirect in enumerate(exchange.redirects, 1):
            sections.append(
                "{}. `{}` → `{}` (`HTTP {}`)".format(
                    index,
                    redact_url(str(redirect.get("from", ""))),
                    redact_url(str(redirect.get("to", ""))),
                    redirect.get("status", "N/A"),
                )
            )
    sections.extend(
        [
            "",
            "Response headers:",
            "",
            "```http",
            render_headers(exchange.response_headers) or "<none>",
            "```",
            "",
            "Response body:",
            "",
            "```json",
            redact_response_body(exchange.response_body) or "<empty>",
            "```",
        ]
    )
    return "\n".join(sections)


def render_terminal_records(runs: List[ProbeRun]) -> str:
    lines = [
        "========== gpt-image-2 测试记录开始 ==========",
        "",
        "- 输出时间: `{}`".format(
            dt.datetime.now().astimezone().isoformat(timespec="seconds")
        ),
        "- Base URL: `{}`".format(BASE_URL),
        "- Authorization: `Bearer REDACTED`",
        (
            "- 说明: 请求与响应记录中的凭证参数会脱敏，"
            "结果 URL 在每次任务末尾原样输出。"
        ),
    ]
    for run in runs:
        lines.extend(
            [
                "",
                "## Run {} — {} / {}".format(run.sequence, run.operation, run.model),
                "",
                "- Started: `{}`".format(run.started_at),
                "- Public/upstream task ID: `{}`".format(run.task_id or "not detected"),
                "- Final status: `{}`".format(run.final_status or "unknown"),
                "",
                "Selected request body:",
                "",
                "```json",
                pretty_json(run.request_body),
                "```",
            ]
        )
        for exchange in run.exchanges:
            lines.extend(["", render_exchange(exchange)])
        if run.result_urls:
            lines.extend(
                [
                    "",
                    "### 结果 URL（原样）",
                    "",
                ]
            )
            lines.extend("- {}".format(url) for url in run.result_urls)
    lines.extend(["", "========== gpt-image-2 测试记录结束 =========="])
    return "\n".join(lines) + "\n"


def choose(title: str, options: List[Tuple[str, str]], default: int = 1) -> str:
    while True:
        print("\n{}".format(title))
        for index, (_, label) in enumerate(options, 1):
            marker = " [默认]" if index == default else ""
            print("  {}. {}{}".format(index, label, marker))
        raw = input("请选择 [{}]: ".format(default)).strip()
        if not raw:
            return options[default - 1][0]
        if raw.isdigit() and 1 <= int(raw) <= len(options):
            return options[int(raw) - 1][0]
        print("输入无效，请输入菜单序号。")


def prompt_required(label: str, default: str = "") -> str:
    while True:
        suffix = " [{}]".format(default) if default else ""
        value = input("{}{}: ".format(label, suffix)).strip()
        if value:
            return value
        if default:
            return default
        print("该字段不能为空。")


def read_clipboard_prompt(pbpaste_path: str = "/usr/bin/pbpaste") -> str:
    try:
        completed = subprocess.run(
            [pbpaste_path],
            capture_output=True,
            check=False,
            encoding="utf-8",
            errors="strict",
            timeout=10,
        )
    except FileNotFoundError as error:
        raise ValueError("未找到 macOS pbpaste，无法读取系统剪贴板") from error
    except subprocess.TimeoutExpired as error:
        raise ValueError("读取系统剪贴板超时") from error
    except UnicodeDecodeError as error:
        raise ValueError("剪贴板内容不是有效的 UTF-8 文本") from error
    if completed.returncode != 0:
        message = completed.stderr.strip() or "未知错误"
        raise ValueError("读取系统剪贴板失败：{}".format(message))
    if not completed.stdout.strip():
        raise ValueError("剪贴板中没有 Prompt 文本")
    return completed.stdout


def prompt_int(label: str, default: int, minimum: int, maximum: int) -> int:
    while True:
        raw = input("{} [{}]: ".format(label, default)).strip()
        if not raw:
            return default
        try:
            value = int(raw)
        except ValueError:
            print("请输入整数。")
            continue
        if minimum <= value <= maximum:
            return value
        print("请输入 {}–{} 之间的整数。".format(minimum, maximum))


def prompt_https_urls(label: str, required: bool, maximum: int) -> List[str]:
    while True:
        raw = input("{}（英文逗号分隔）: ".format(label)).strip()
        values = [item.strip() for item in raw.split(",") if item.strip()]
        if not values and not required:
            return []
        if not values:
            print("至少需要一个 HTTPS URL。")
            continue
        if len(values) > maximum:
            print("最多允许 {} 个 URL。".format(maximum))
            continue
        if any(urllib.parse.urlsplit(value).scheme.lower() != "https" for value in values):
            print("所有素材都必须使用 HTTPS URL。")
            continue
        return values


def prompt_size(model: str) -> str:
    default_size = str(MODELS[model]["default_size"])
    selected = choose(
        "选择尺寸",
        [
            (default_size, "模型默认精确尺寸：{}".format(default_size)),
            ("1:1", "比例 1:1"),
            ("16:9", "比例 16:9"),
            ("9:16", "比例 9:16"),
            ("custom", "自定义精确尺寸或比例"),
        ],
    )
    if selected != "custom":
        return validate_size(model, selected)
    while True:
        raw = prompt_required("自定义尺寸", default_size)
        try:
            return validate_size(model, raw)
        except ValueError as error:
            print("尺寸无效：{}".format(error))


def collect_request(operation: str) -> Tuple[str, Dict[str, Any]]:
    model = choose(
        "选择模型",
        [(name, name) for name in MODELS],
    )
    while True:
        input(
            "请先把完整 Prompt 复制到 macOS 系统剪贴板，"
            "准备好后按回车读取（不要粘贴到终端）: "
        )
        try:
            prompt = read_clipboard_prompt()
            break
        except ValueError as error:
            print("Prompt 读取失败：{}".format(error))
    print(
        "已读取 Prompt：{} 个字符，{} 行".format(
            len(prompt), prompt.count("\n") + 1
        )
    )
    size = prompt_size(model)
    quality = choose("选择 quality", [("medium", "medium"), ("low", "low")])
    background = choose(
        "选择 background",
        [("auto", "auto"), ("opaque", "opaque"), ("transparent", "transparent")],
    )
    count = prompt_int("生成数量 n", 1, 1, 10)
    response_format = choose(
        "选择 response_format",
        [
            ("url", "url（推荐，便于检查结果 URL）"),
            ("b64_json", "b64_json（探测上游是否支持）"),
        ],
    )
    images: Optional[List[str]] = None
    mask = ""
    if operation == "edit":
        images = prompt_https_urls("参考图 HTTPS URL", True, 6)
        mask_values = prompt_https_urls("Mask HTTPS URL（可留空）", False, 1)
        mask = mask_values[0] if mask_values else ""
    return model, build_request_body(
        operation=operation,
        model=model,
        prompt=prompt,
        size=size,
        quality=quality,
        background=background,
        count=count,
        response_format=response_format,
        images=images,
        mask=mask,
    )


def confirm_paid_request(body: Dict[str, Any]) -> bool:
    print("\n即将发送以下付费请求（失败或超时后不会自动重放 POST）：")
    print(pretty_json(body))
    answer = input("确认发送？输入 YES 继续: ").strip()
    return answer == "YES"


def poll_task(
    run: ProbeRun,
    api_key: str,
    interval: int,
    timeout_seconds: int,
) -> Any:
    path_segment = "generations" if run.operation == "generation" else "edits"
    path = "/v1/images/{}/{}".format(path_segment, urllib.parse.quote(run.task_id, safe=""))
    deadline = time.monotonic() + timeout_seconds
    attempt = 0
    consecutive_errors = 0
    while time.monotonic() < deadline:
        attempt += 1
        exchange, payload = request_json(
            "轮询任务 #{}".format(attempt), "GET", path, api_key, timeout=60.0
        )
        run.exchanges.append(exchange)
        status = extract_status(payload)
        classification = classify_status(status)
        print(
            "轮询 #{:<3} HTTP {:<4} status={}".format(
                attempt,
                exchange.status if exchange.status is not None else "N/A",
                status or "未识别",
            )
        )
        if exchange.error or is_retryable_poll_status(exchange.status):
            consecutive_errors += 1
            if consecutive_errors >= 5:
                run.final_status = "polling_error"
                return None
        else:
            consecutive_errors = 0
        permanent_http_error = (
            exchange.status is not None
            and exchange.status >= 400
            and not is_retryable_poll_status(exchange.status)
        )
        if permanent_http_error:
            run.final_status = "polling_http_{}".format(exchange.status)
            return None
        if classification == "success":
            run.final_status = status
            return payload
        if classification == "failure":
            run.final_status = status
            return payload
        time.sleep(interval)
    run.final_status = "polling_timeout"
    return None


def run_probe(
    sequence: int,
    operation: str,
    api_key: str,
    interval: int,
    timeout_seconds: int,
) -> Optional[ProbeRun]:
    model, body = collect_request(operation)
    if not confirm_paid_request(body):
        print("已取消，没有发送请求。")
        return None
    run = ProbeRun(
        sequence=sequence,
        operation=operation,
        model=model,
        request_body=body,
        started_at=dt.datetime.now().astimezone().isoformat(timespec="seconds"),
    )
    endpoint = "/v1/images/generations" if operation == "generation" else "/v1/images/edits"
    try:
        exchange, payload = request_json("创建任务", "POST", endpoint, api_key, body)
    except KeyboardInterrupt:
        run.final_status = "interrupted_during_submission"
        print(
            "\n提交等待被中断。请求可能已经到达上游，"
            "脚本不会自动重试。"
        )
        return run
    run.exchanges.append(exchange)
    print(
        "创建请求完成：HTTP {}，耗时 {:.3f}s".format(
            exchange.status if exchange.status is not None else "N/A",
            exchange.elapsed_seconds,
        )
    )
    run.task_id = extract_task_id(payload)
    if not run.task_id:
        run.final_status = "task_id_not_detected"
        print("未从创建响应中识别出任务 ID；完整响应稍后输出到终端。")
        return run
    print("任务 ID：{}".format(run.task_id))
    try:
        final_payload = poll_task(run, api_key, interval, timeout_seconds)
    except KeyboardInterrupt:
        run.final_status = "polling_interrupted"
        print("\n已停止轮询；上游任务不会被取消，也不会自动重试提交。")
        return run
    if classify_status(run.final_status) != "success":
        print("任务未成功结束：{}".format(run.final_status))
        return run
    run.result_urls = extract_result_urls(final_payload)
    if run.result_urls:
        print("任务完成，获得 {} 个结果 URL：".format(len(run.result_urls)))
        for result_url in run.result_urls:
            print(result_url)
    else:
        print("任务完成，但最终响应中没有识别到结果 URL。")
    return run


def read_api_key() -> str:
    while True:
        value = getpass.getpass(
            "请输入上游 API Key（仅在本次运行内存中使用，不会写入磁盘）: "
        ).strip()
        if value:
            return value
        print("API Key 不能为空。")


def main() -> int:
    print("gpt-image-2 上游交互测试器")
    print("Base URL: {}".format(BASE_URL))
    print("注意：生成 POST 不自动重试；发送前必须输入 YES 确认。")
    api_key = read_api_key()
    runs: List[ProbeRun] = []
    interval = 3
    timeout_seconds = 15 * 60
    try:
        while True:
            action = choose(
                "主菜单",
                [
                    ("generation", "测试文生图"),
                    ("edit", "测试图片编辑"),
                    ("settings", "修改轮询设置"),
                    ("records", "在终端重新输出全部测试记录"),
                    ("exit", "退出并输出最终测试记录"),
                ],
            )
            if action in {"generation", "edit"}:
                run = run_probe(
                    len(runs) + 1,
                    action,
                    api_key,
                    interval,
                    timeout_seconds,
                )
                if run is not None:
                    runs.append(run)
                    print(render_terminal_records([run]))
            elif action == "settings":
                interval = prompt_int("轮询间隔（秒）", interval, 1, 60)
                timeout_seconds = prompt_int(
                    "最长轮询时间（秒）", timeout_seconds, 30, 7200
                )
            elif action == "records":
                if runs:
                    print(render_terminal_records(runs))
                else:
                    print("当前还没有测试记录。")
            else:
                break
    except KeyboardInterrupt:
        print("\n收到中断，正在输出已经采集的测试记录。")
    finally:
        if runs:
            print(render_terminal_records(runs))
        else:
            print("本次没有发送请求。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
