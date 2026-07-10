import type { ReactNode } from "react";
import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://openintern.dev"),
  title: {
    default: "OpenIntern — open tech internship corpus",
    template: "%s",
  },
  description:
    "Free structured tech internships with a public API, no-account board, and community company registry.",
  openGraph: {
    siteName: "OpenIntern",
    type: "website",
    title: "OpenIntern — open tech internship corpus",
    description:
      "Free structured tech internships with a public API, no-account board, and community company registry.",
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=Inter:wght@400;500;600;700&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <header className="site-header">
          <div className="container inner">
            <a className="brand" href="/">
              <span className="brand-mark" aria-hidden="true" />
              Open<span>Intern</span>
            </a>
            <nav className="nav">
              <div className="nav-links">
                <a href="/">Jobs</a>
                <a href="/health">Health</a>
                <a href="/docs">Docs</a>
                <a href="https://github.com/dnexdev/openintern">GitHub</a>
              </div>
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
        <footer className="footer">
          <div className="container">
            OpenIntern is free and open source (Apache-2.0). Listings are never
            paywalled. Apply on the employer site. Daily dumps and API docs at{" "}
            <a href="/docs">/docs</a>. Mark applications on this device only — no
            account required.
          </div>
        </footer>
      </body>
    </html>
  );
}
