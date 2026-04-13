#!/usr/bin/env python3
import argparse
import json
import os
import sys


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", required=True)
    parser.add_argument("--lang", default="ch")
    return parser.parse_args()


def main():
    args = parse_args()

    try:
        from paddleocr import PaddleOCR
    except Exception as exc:
        raise RuntimeError(
            "PaddleOCR is not installed. Install it with `pip install paddleocr paddlepaddle`."
        ) from exc

    ocr = PaddleOCR(use_angle_cls=True, lang=args.lang)
    raw = ocr.ocr(args.image, cls=True)

    lines = []
    for page in raw or []:
        for item in page or []:
            box = item[0]
            text = item[1][0]
            score = float(item[1][1])
            lines.append(
                {
                    "text": text.strip(),
                    "score": score,
                    "box": box,
                }
            )

    payload = {
        "fileName": os.path.basename(args.image),
        "lines": lines,
    }
    sys.stdout.write(json.dumps(payload, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        sys.stderr.write(str(exc))
        sys.exit(1)
