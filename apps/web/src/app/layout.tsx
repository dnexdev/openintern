import type { ReactNode } from "react";
import type { Metadata } from "next";
import "./globals.css";

// Vercel Web Analytics: a same-origin script, no cookies, page views only.
// Enable Analytics on the Vercel project, then set OPENINTERN_ANALYTICS=vercel.
const analyticsEnabled = process.env.OPENINTERN_ANALYTICS?.trim() === "vercel";

export const metadata: Metadata = {
  metadataBase: new URL("https://openintern.dev"),
  title: {
    default: "OpenIntern — free open-source tech internships",
    template: "%s · OpenIntern",
  },
  description:
    "Free tech internship board with no account, a public API, and daily dumps. Open source (Apache-2.0). Never paywalled.",
  openGraph: {
    siteName: "OpenIntern",
    type: "website",
    title: "OpenIntern — free open-source tech internships",
    description:
      "Free tech internship board with no account, a public API, and daily dumps. Open source. Never paywalled.",
    url: "https://openintern.dev",
    images: [
      {
        url: "/opengraph-image",
        width: 1200,
        height: 630,
        alt: "OpenIntern",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "OpenIntern — free open-source tech internships",
    description:
      "Free tech internship board. No account. Public API and daily dumps. Open source.",
    images: ["/twitter-image"],
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
        <link
          rel="preload"
          href="/fonts/IBMPlexSans-Regular-Latin1.woff2"
          as="font"
          type="font/woff2"
          crossOrigin=""
        />
        {analyticsEnabled ? <script defer src="/_vercel/insights/script.js" /> : null}
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
                <a href="/jobs">Jobs</a>
                <a href="/docs">Docs</a>
                <a href="https://github.com/dnexdev/openintern">GitHub</a>
              </div>
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
        <footer className="footer">
          <div className="container footer-inner">
            <p>
              Free and open source (Apache-2.0). Never paywalled. Apply on the
              employer site. No account required.
            </p>
            <p className="footer-links">
              <a href="/docs">Docs</a>
              <a href="/vs/intern-insider">Compare</a>
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
