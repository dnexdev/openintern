import { auth, signIn } from "@/auth";
import { redirect } from "next/navigation";

export default async function LoginPage() {
  const session = await auth().catch(() => null);
  if (session?.user) redirect("/account");

  const githubReady = Boolean(process.env.AUTH_GITHUB_ID && process.env.AUTH_GITHUB_SECRET);
  const googleReady = Boolean(process.env.AUTH_GOOGLE_ID && process.env.AUTH_GOOGLE_SECRET);

  return (
    <>
      <section className="hero">
        <h1>Sign in (optional)</h1>
        <p>
          Browse jobs with no account. Sign in only if you want saved jobs and
          alert digests.
        </p>
      </section>

      <div className="panel" style={{ maxWidth: 420 }}>
        {!githubReady && !googleReady ? (
          <p className="empty">
            Auth providers are not configured. Set{" "}
            <code className="mono">AUTH_GITHUB_*</code> or{" "}
            <code className="mono">AUTH_GOOGLE_*</code> in env.
          </p>
        ) : (
          <div style={{ display: "grid", gap: "0.75rem" }}>
            {githubReady ? (
              <form
                action={async () => {
                  "use server";
                  await signIn("github", { redirectTo: "/account" });
                }}
              >
                <button className="btn btn-primary" type="submit" style={{ width: "100%" }}>
                  Continue with GitHub
                </button>
              </form>
            ) : null}
            {googleReady ? (
              <form
                action={async () => {
                  "use server";
                  await signIn("google", { redirectTo: "/account" });
                }}
              >
                <button className="btn" type="submit" style={{ width: "100%" }}>
                  Continue with Google
                </button>
              </form>
            ) : null}
          </div>
        )}
      </div>
    </>
  );
}
