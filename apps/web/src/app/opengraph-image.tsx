import { ImageResponse } from "next/og";

export const runtime = "edge";
export const alt = "OpenIntern — open tech internship corpus";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OpenGraphImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          padding: "64px 72px",
          background: "linear-gradient(145deg, #0b1220 0%, #12203a 48%, #0f3f8c 100%)",
          color: "#f6f7f9",
          fontFamily: "ui-sans-serif, system-ui, sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 18 }}>
          <div
            style={{
              width: 52,
              height: 52,
              borderRadius: 12,
              border: "4px solid transparent",
              background:
                "linear-gradient(#0b1220, #0b1220) padding-box, linear-gradient(135deg, #f5c518, #7dd3fc, #22d3ee) border-box",
            }}
          />
          <div style={{ display: "flex", fontSize: 42, fontWeight: 700, letterSpacing: -1 }}>
            <span style={{ display: "flex" }}>Open</span>
            <span style={{ display: "flex", color: "#7dd3fc" }}>Intern</span>
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 18, maxWidth: 920 }}>
          <div
            style={{
              display: "flex",
              fontSize: 58,
              fontWeight: 700,
              lineHeight: 1.1,
              letterSpacing: -1.5,
            }}
          >
            Tech internships, open by default
          </div>
          <div style={{ display: "flex", fontSize: 28, color: "#c5d0e0", lineHeight: 1.35 }}>
            Free structured corpus · public API · no-account board
          </div>
        </div>

        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            fontSize: 24,
            color: "#9db0c9",
          }}
        >
          <span>openintern.dev</span>
          <span>Apache-2.0</span>
        </div>
      </div>
    ),
    { ...size },
  );
}
