"""Owner-run Claude text pilot. Requires Python 3 and a running RouteInlet."""
import json
import os
import secrets
import urllib.error
import urllib.request


def turn(messages):
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
    finished = False
    with urllib.request.urlopen(request, timeout=130) as response:
        for raw in response:
            line = raw.decode().strip()
            if not line.startswith("data:"):
                continue
            data = line[5:].strip()
            if data == "[DONE]":
                if not finished or not fragments:
                    raise ValueError("stream terminated without text or finish reason")
                print("request", response.headers["Gateway-Request-Id"],
                      "reported models", sorted(models), "text deltas", len(fragments),
                      "funding", response.headers["Gateway-Billing-Source"])
                return "".join(fragments)
            chunk = json.loads(data)
            if "error" in chunk:
                raise ValueError("upstream stream error")
            if chunk.get("model"):
                models.add(chunk["model"])
            for choice in chunk.get("choices", []):
                if choice["delta"].get("content"):
                    fragments.append(choice["delta"]["content"])
                if choice.get("finish_reason") is not None:
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
    print("Two-turn application check passed. Verify included usage separately.")


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as exc:
        raise SystemExit(f"HTTP {exc.code}: {exc.read(1024).decode(errors='replace')}") from None
