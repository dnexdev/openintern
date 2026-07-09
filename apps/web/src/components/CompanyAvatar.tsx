"use client";

import { useState } from "react";

const AVATAR_COLORS = [
  "#1454b4",
  "#0a7a3d",
  "#a13ea1",
  "#c0392b",
  "#b7791f",
  "#2f6f6f",
  "#5a4fcf",
  "#c2410c",
];

function initials(name: string) {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "?";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return (words[0][0] + words[1][0]).toUpperCase();
}

function avatarColor(name: string) {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash << 5) - hash + name.charCodeAt(i);
    hash |= 0;
  }
  return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

const BOARD_SUBDOMAINS = /^(careers|jobs|apply|boards|www)\./i;

function companyDomain(
  name: string,
  websiteUrl?: string | null,
  careersUrl?: string | null,
): string {
  for (const url of [websiteUrl, careersUrl]) {
    if (!url) continue;
    try {
      const host = new URL(url).hostname.replace(BOARD_SUBDOMAINS, "");
      // ATS-hosted career pages (jobs.smartrecruiters.com/Foo) are not the
      // company's own domain — skip those and fall through.
      if (/greenhouse\.io|lever\.co|ashbyhq\.com|workable\.com|smartrecruiters\.com/i.test(host)) {
        continue;
      }
      if (host) return host;
    } catch {
      // malformed URL in registry data — fall through to heuristic
    }
  }
  return `${name.toLowerCase().replace(/[^a-z0-9]/g, "")}.com`;
}

export function CompanyAvatar({
  name,
  websiteUrl,
  careersUrl,
}: {
  name: string;
  websiteUrl?: string | null;
  careersUrl?: string | null;
}) {
  const [logoFailed, setLogoFailed] = useState(false);
  const domain = companyDomain(name, websiteUrl, careersUrl);

  return (
    <div
      className="avatar"
      style={logoFailed ? { background: avatarColor(name) } : { background: "#ffffff", border: "1px solid var(--border)" }}
      aria-hidden="true"
    >
      {logoFailed ? (
        initials(name)
      ) : (
        <img
          src={`https://logos.hunter.io/${domain}`}
          alt=""
          width={44}
          height={44}
          loading="lazy"
          style={{ width: "100%", height: "100%", objectFit: "contain", borderRadius: "inherit" }}
          onError={() => setLogoFailed(true)}
        />
      )}
    </div>
  );
}
