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

/**
 * Brand domains when registry website_url is missing or wrong.
 * Prefer real marketing domains — guessed `{name}.com` is often wrong
 * (e.g. IMC Trading → imctrading.com, Scale AI → scaleai.com).
 */
const DOMAIN_OVERRIDES: Record<string, string> = {
  // AI / infra
  cohere: "cohere.com",
  openai: "openai.com",
  anthropic: "anthropic.com",
  databricks: "databricks.com",
  snowflake: "snowflake.com",
  "scale-ai": "scale.com",
  scaleai: "scale.com",
  perplexity: "perplexity.ai",
  "hugging-face": "huggingface.co",
  huggingface: "huggingface.co",
  mistral: "mistral.ai",
  anduril: "anduril.com",
  // Consumer / product
  stripe: "stripe.com",
  cloudflare: "cloudflare.com",
  figma: "figma.com",
  notion: "notion.so",
  airbnb: "airbnb.com",
  rippling: "rippling.com",
  replit: "replit.com",
  bunq: "bunq.com",
  // Trading / finance
  imc: "imc.com",
  "jump-trading": "jumptrading.com",
  jumptrading: "jumptrading.com",
  "jane-street": "janestreet.com",
  janestreet: "janestreet.com",
  citadel: "citadel.com",
  "two-sigma": "twosigma.com",
  hrt: "hudsonrivertrading.com",
  // Other frequent misses
  "space-x": "spacex.com",
  spacex: "spacex.com",
  block: "block.xyz",
  square: "block.xyz",
  "1password": "1password.com",
  "cursor": "cursor.com",
};

/** Direct logo URLs when favicon CDNs return generic placeholders. */
const LOGO_URL_OVERRIDES: Record<string, string[]> = {
  anduril: ["/logos/anduril.png"],
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

function logoCandidates(domain: string, slug?: string | null): string[] {
  const direct = slug ? (LOGO_URL_OVERRIDES[slug] ?? []) : [];
  // DuckDuckGo often has fuller brand icons; Google is a solid fallback.
  // icon.horse last — it HTTP-200s a generic glyph for unknown brands.
  const d = encodeURIComponent(domain);
  return [
    ...direct,
    `https://icons.duckduckgo.com/ip3/${d}.ico`,
    `https://www.google.com/s2/favicons?domain=${d}&sz=128`,
    `https://icon.horse/icon/${d}`,
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
  const candidates = logoCandidates(domain, slug);
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
            padding: "4px",
          }}
          onError={() => setIdx((i) => i + 1)}
        />
      )}
    </div>
  );
}
