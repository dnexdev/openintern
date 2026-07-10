import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ImageResponse } from "next/og";

export const ogImageAlt = "OpenIntern";
export const ogImageSize = { width: 1200, height: 630 };
export const ogImageContentType = "image/png";

const fontPath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../assets/fonts/IBMPlexSans-Bold.ttf",
);

let fontDataPromise: Promise<ArrayBuffer> | null = null;

function loadIbmPlexSansBold(): Promise<ArrayBuffer> {
  if (!fontDataPromise) {
    fontDataPromise = readFile(fontPath).then(
      (data) => data.buffer.slice(data.byteOffset, data.byteOffset + data.byteLength) as ArrayBuffer,
    );
  }
  return fontDataPromise;
}

export async function createWordmarkOgImage(): Promise<ImageResponse> {
  const fontData = await loadIbmPlexSansBold();

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#0b1220",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 28,
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 96,
              height: 96,
              borderRadius: 22,
              background:
                "linear-gradient(135deg, #f5c518 0%, #7dd3fc 45%, #22d3ee 100%)",
            }}
          >
            <div
              style={{
                width: 72,
                height: 72,
                borderRadius: 14,
                background: "#0b1220",
              }}
            />
          </div>
          <div
            style={{
              display: "flex",
              fontFamily: "IBM Plex Sans",
              fontSize: 96,
              fontWeight: 700,
              letterSpacing: -3.2,
            }}
          >
            <span style={{ display: "flex", color: "#f6f7f9" }}>Open</span>
            <span style={{ display: "flex", color: "#7dd3fc" }}>Intern</span>
          </div>
        </div>
      </div>
    ),
    {
      ...ogImageSize,
      fonts: [
        {
          name: "IBM Plex Sans",
          data: fontData,
          style: "normal",
          weight: 700,
        },
      ],
    },
  );
}
