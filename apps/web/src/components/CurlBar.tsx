"use client";

import { useState } from "react";

const CURL_CMD = `curl "https://openintern.dev/api/v1/jobs?role=software&limit=5"`;

export function CurlBar() {
  const [copied, setCopied] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(CURL_CMD);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div className="curl-bar-wrap">
      <div className="curl-bar">
        <code className="curl-bar-cmd" aria-label="API curl example">
          {CURL_CMD}
        </code>
        <button
          type="button"
          className="curl-bar-copy"
          onClick={copy}
          aria-label={copied ? "Copied" : "Copy curl command"}
        >
          {copied ? (
            <span className="curl-bar-copy-label">Copied</span>
          ) : (
            <svg
              width="16"
              height="16"
              viewBox="0 0 16 16"
              fill="none"
              aria-hidden="true"
            >
              <rect
                x="5.5"
                y="5.5"
                width="8"
                height="8"
                rx="1.5"
                stroke="currentColor"
                strokeWidth="1.4"
              />
              <path
                d="M3.5 10.5V3.5A1 1 0 0 1 4.5 2.5h7"
                stroke="currentColor"
                strokeWidth="1.4"
                strokeLinecap="round"
              />
            </svg>
          )}
        </button>
      </div>
      <p className="curl-bar-secondary">
        Or read the <a href="/docs">API docs</a>
      </p>
    </div>
  );
}
