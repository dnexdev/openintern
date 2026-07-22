/**
 * Shared optional Playwright loader (not a hard package dependency for Hobby CI).
 */
const UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36";

export { UA as BROWSER_UA };

export type PlaywrightPage = {
  goto: (
    url: string,
    opts?: { waitUntil?: string; timeout?: number },
  ) => Promise<{ status: () => number } | null>;
  content: () => Promise<string>;
  evaluate: <T>(fn: (arg: string) => Promise<T>, arg: string) => Promise<T>;
  waitForTimeout: (ms: number) => Promise<void>;
};

export type PlaywrightBrowser = {
  newPage: (opts?: { userAgent?: string }) => Promise<PlaywrightPage>;
  close: () => Promise<void>;
};

export async function loadPlaywrightChromium(): Promise<{
  launch: (opts?: { headless?: boolean }) => Promise<PlaywrightBrowser>;
}> {
  try {
    const mod = (await new Function("return import('playwright')")()) as {
      chromium: {
        launch: (opts?: { headless?: boolean }) => Promise<PlaywrightBrowser>;
      };
    };
    return mod.chromium;
  } catch {
    throw new Error(
      "OPENINTERN_BROWSER=1 but playwright is not installed. " +
        "Run `pnpm add -Dw playwright && pnpm exec playwright install chromium`, " +
        "or set the corresponding OPENINTERN_*_PATH dump env var. See CONTRIBUTING.",
    );
  }
}

export function browserEnabled(): boolean {
  return process.env.OPENINTERN_BROWSER === "1";
}
