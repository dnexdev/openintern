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

/**
 * Initials only. Logos used to load from the company site, DuckDuckGo, Google
 * and icon.horse straight from each visitor's browser, which told those
 * services who browses OpenIntern. Logos come back as build-time assets
 * served from this origin (plan phase W).
 */
export function CompanyAvatar({
  name,
}: {
  name: string;
  websiteUrl?: string | null;
  careersUrl?: string | null;
  slug?: string | null;
}) {
  return (
    <div className="avatar" style={{ background: avatarColor(name) }} aria-hidden="true">
      {initials(name)}
    </div>
  );
}
