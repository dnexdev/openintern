import type { ReactNode } from "react";
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://openintern.dev"),
  title: {
    default: "OpenIntern — open tech internship corpus",
    template: "%s · OpenIntern",
  },
  description:
    "Free structured tech internships with a public API, no-account board, and community company registry. Never paywalled.",
  openGraph: {
    siteName: "OpenIntern",
    type: "website",
    title: "OpenIntern — open tech internship corpus",
    description:
      "Free structured tech internships with a public API, no-account board, and community company registry.",
    url: "https://openintern.dev",
  },
  twitter: {
    card: "summary_large_image",
    title: "OpenIntern — open tech internship corpus",
    description:
      "Free structured tech internships. Public API, daily dumps, no account required.",
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:wght@400;500;600;700&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <header className="site-header">
          <div className="container inner">
            <a className="brand" href="/">
              <span className="brand-mark" aria-hidden="true" />
              <span className="brand-name">
                Open<span>Intern</span>
              </span>
            </a>
            <nav className="nav">
              <div className="nav-links">
                <a href="/">Jobs</a>
                <a href="/docs">Docs</a>
                <a href="/health">Health</a>
                <a href="https://github.com/dnexdev/openintern">GitHub</a>
              </div>
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
        <footer className="footer">
          <div className="container footer-inner">
            <p>
              OpenIntern is free and open source (Apache-2.0). Listings are never
              paywalled. Apply on the employer site. Not affiliated with listed
              companies. Mark applications on this device only — no account
              required.
            </p>
            <p className="footer-links">
              <a href="/docs">Docs</a>
              <a href="/privacy">Privacy</a>
              <a href="/health">Health</a>
              <a href="https://github.com/dnexdev/openintern">GitHub</a>
              <a href="https://github.com/dnexdev/openintern/releases/tag/dump-latest">
                Dumps
              </a>
            </p>
          </div>
        </footer>
      </body>
    </html>
  );
}
