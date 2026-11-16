"""Owner-run Claude text pilot. Requires Python 3 and a running RouteInlet."""
import io
import json
import os
import secrets
import time
import urllib.error
import urllib.request
from http.client import HTTPException


def events(response):
    """Dispatch complete SSE data events. A blank line completes the event."""
    fields = []
    with io.TextIOWrapper(response, encoding="utf-8-sig", newline=None) as lines:
        for line in lines:
            line = line.rstrip("\n")
            if not line:
                if fields:
                    yield "\n".join(fields)
                    fields.clear()
                continue
            field, _, value = line.partition(":")
            if field == "data":
                fields.append(value[1:] if value.startswith(" ") else value)


def check_arrivals(chars, arrivals):
    # Only non-whitespace text participates. Headers, keepalives, finish
    # events, and a delayed [DONE] cannot make a buffered answer pass.
    span = arrivals[-1] - arrivals[0] if arrivals else 0
    print(f"Long-output timing: {chars} characters, {len(arrivals)} text events, "
          f"{span:.3f}s from first to last text.")
    if len(arrivals) >= 3:
        print(f"Text arrival offsets (seconds): first 0.000, "
              f"middle {arrivals[len(arrivals) // 2] - arrivals[0]:.3f}, last {span:.3f}")
    if chars < 1000:
        raise ValueError("long-output fixture produced fewer than 1000 characters")
    if not any(t - arrivals[0] >= 0.1 and arrivals[-1] - t >= 0.1 for t in arrivals[1:-1]):
        raise ValueError("incremental delivery inconclusive: need three text arrivals "
                         "spaced at least 100 ms apart")


def turn(messages, measure_stream=False):
    model = os.environ["ROUTEINLET_MODEL"]
    payload = {"model": model, "messages": messages, "stream": True}
    request = urllib.request.Request(
        "http://127.0.0.1:8787/v1/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Authorization": "Bearer " + os.environ["ROUTEINLET_KEY"],
                 "Content-Type": "application/json"},
    )
    fragments = []
    models = set()
    arrivals = []
    finished = False
    with urllib.request.urlopen(request, timeout=130) as response:
        print("request", response.headers["Gateway-Request-Id"],
              "funding", response.headers["Gateway-Billing-Source"])
        for data in events(response):
            if data == "[DONE]":
                if not finished or not fragments:
                    raise ValueError("stream terminated without text or a normal stop")
                answer = "".join(fragments)
                print("reported models", sorted(models), "text deltas", len(fragments))
                if measure_stream:
                    check_arrivals(len(answer.strip()), arrivals)
                return answer
            chunk = json.loads(data)
            if "error" in chunk:
                raise ValueError("upstream stream error")
            if chunk.get("model"):
                models.add(chunk["model"])
            choices = chunk.get("choices", [])
            if len(choices) > 1:
                raise ValueError("unexpected extra choice in single-answer fixture")
            for choice in choices:
                if choice.get("index", 0) != 0:
                    raise ValueError("unexpected extra choice in single-answer fixture")
                reason = choice.get("finish_reason")
                if reason == "length":
                    raise ValueError("truncated output (finish_reason=length)")
                if reason == "content_filter":
                    raise ValueError("filtered output (finish_reason=content_filter)")
                if reason is not None and reason != "stop":
                    raise ValueError(f"unexpected finish_reason={reason!r}")
                text = choice.get("delta", {}).get("content")
                if text:
                    if finished:
                        raise ValueError("text arrived after finish_reason=stop")
                    fragments.append(text)
                    if measure_stream and text.strip():
                        arrivals.append(time.monotonic())
                if reason == "stop":
                    finished = True
    raise ValueError("stream closed before [DONE]")


def main():
    reference = secrets.token_hex(6)
    history = [
        {"role": "system", "content": "Follow the requested answer format exactly."},
        {"role": "user", "content": f"My reference is {reference}. Reply only with OK."},
    ]
    first = turn(history)
    if first.strip() != "OK":
        raise ValueError("first turn failed its answer-format check")
    history.extend([{"role": "assistant", "content": first},
                    {"role": "user", "content": "Reply only with my reference."}])
    if turn(history).strip() != reference:
        raise ValueError("second turn failed the history check")
    print("Two-turn history passed.")
    turn([{"role": "user", "content": "Write a 400-word guide to growing tomatoes "
           "from seed. Cover planting, watering, and transplanting."}], measure_stream=True)
    print("Incremental text observed at the application. Verify included usage separately.")


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as exc:
        raise SystemExit(f"HTTP {exc.code}: {exc.read(1024).decode(errors='replace')}") from None
    except (HTTPException, OSError, ValueError) as exc:
        raise SystemExit(f"Probe failed: {exc}") from None
