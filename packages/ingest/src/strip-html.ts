/**
 * Plain-text extraction from HTML/markdown cell noise for ingest matching.
 * Not a browser XSS sanitizer — output is never re-embedded as HTML.
 */

/** Strip tags and decode common entities (&amp; last to avoid double-unescape). */
export function stripHtml(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, " ")
    .replace(/<\/?[a-zA-Z][^>]*>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&#0?39;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, "&")
    .replace(/^[\s\p{Extended_Pictographic}\uFE0F\u200D🔥🔒🛂🇺🇸🎓]+/gu, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** Job description HTML → plain text (keeps paragraph/break newlines). */
export function stripHtmlTags(html: string): string {
  return html
    .replace(/<br\s*\/?>/gi, "\n")
    .replace(/<\/p>/gi, "\n")
    .replace(/<\/?[a-zA-Z][^>]*>/g, " ")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/\s+/g, " ")
    .trim();
}
