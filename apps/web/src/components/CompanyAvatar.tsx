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

const BOARD_SUBDOMAINS = /^(careers|jobs|apply|boards|www|ats)\./i;

const ATS_HOST =
  /greenhouse\.io|lever\.co|ashbyhq\.com|workable\.com|smartrecruiters\.com|recruitee\.com|rippling\.com|bamboohr\.com/i;

/** Known brand domains when registry website_url is missing. */
const DOMAIN_OVERRIDES: Record<string, string> = {
  cohere: "cohere.com",
  stripe: "stripe.com",
  openai: "openai.com",
  anthropic: "anthropic.com",
  databricks: "databricks.com",
  cloudflare: "cloudflare.com",
  figma: "figma.com",
  notion: "notion.so",
  airbnb: "airbnb.com",
  "hugging-face": "huggingface.co",
  huggingface: "huggingface.co",
  rippling: "rippling.com",
  bunq: "bunq.com",
  replit: "replit.com",
  perplexity: "perplexity.ai",
};

function companyDomain(
  name: string,
  websiteUrl?: string | null,
  careersUrl?: string | null,
  slug?: string | null,
): string {
  if (slug && DOMAIN_OVERRIDES[slug]) return DOMAIN_OVERRIDES[slug];

  for (const url of [websiteUrl, careersUrl]) {
    if (!url) continue;
    try {
      const host = new URL(url).hostname.replace(BOARD_SUBDOMAINS, "");
      if (ATS_HOST.test(host)) continue;
      if (host) return host;
    } catch {
      // malformed URL — fall through
    }
  }

  const key = name.toLowerCase().replace(/[^a-z0-9]/g, "");
  if (DOMAIN_OVERRIDES[key]) return DOMAIN_OVERRIDES[key];
  return `${key}.com`;
}

function logoCandidates(domain: string): string[] {
  // icon.horse tends to return real brand marks; Google favicon as fallback.
  // Avoid logos.hunter.io — often returns low-quality / wrong glyphs (e.g. Cohere).
  return [
    `https://icon.horse/icon/${encodeURIComponent(domain)}`,
    `https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=128`,
  ];
}

export function CompanyAvatar({
  name,
  websiteUrl,
  careersUrl,
  slug,
}: {
  name: string;
  websiteUrl?: string | null;
  careersUrl?: string | null;
  slug?: string | null;
}) {
  const domain = companyDomain(name, websiteUrl, careersUrl, slug);
  const candidates = logoCandidates(domain);
  const [idx, setIdx] = useState(0);
  const failed = idx >= candidates.length;
  const src = candidates[idx];

  return (
    <div
      className="avatar"
      style={
        failed
          ? { background: avatarColor(name) }
          : { background: "#ffffff", border: "1px solid var(--border)" }
      }
      aria-hidden="true"
    >
      {failed || !src ? (
        initials(name)
      ) : (
        <img
          key={src}
          src={src}
          alt=""
          width={44}
          height={44}
          loading="lazy"
          referrerPolicy="no-referrer"
          style={{
            width: "100%",
            height: "100%",
            objectFit: "contain",
            borderRadius: "inherit",
          }}
          onError={() => setIdx((i) => i + 1)}
        />
      )}
    </div>
  );
}
