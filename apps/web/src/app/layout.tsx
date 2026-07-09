import type { ReactNode } from "react";
import { auth } from "@/auth";
import "./globals.css";

export const metadata = {
  title: "OpenIntern — open tech internship corpus",
  description:
    "Free structured tech internships with a public API, no-account board, and community company registry.",
};

export default async function RootLayout({ children }: { children: ReactNode }) {
  const session = await auth().catch(() => null);

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
              Open<span>Intern</span>
            </a>
            <nav className="nav">
              <a href="/">Jobs</a>
              <a href="/health">Health</a>
              <a href="/api/v1/jobs">API</a>
              <a href="https://github.com/dnexdev/openintern">GitHub</a>
              {session?.user ? (
                <>
                  <a href="/account">Account</a>
                  <a className="btn" href="/api/auth/signout">
                    Sign out
                  </a>
                </>
              ) : (
                <a className="btn" href="/login">
                  Sign in
                </a>
              )}
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
        <footer className="footer">
          <div className="container">
            OpenIntern is free and open source (Apache-2.0). Listings are never
            paywalled. Apply on the employer site.
          </div>
        </footer>
      </body>
    </html>
  );
}
