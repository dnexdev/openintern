package web

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
)

func (s *server) robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	_, _ = fmt.Fprintf(w, "User-Agent: *\nAllow: /\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", s.baseURL)
}

func xmlText(buf *bytes.Buffer, value string) {
	_ = xml.EscapeText(buf, []byte(value))
}

func sitemapURL(buf *bytes.Buffer, location, modified, frequency, priority string) {
	buf.WriteString("<url>\n<loc>")
	xmlText(buf, location)
	buf.WriteString("</loc>\n")
	if modified != "" {
		buf.WriteString("<lastmod>")
		xmlText(buf, modified)
		buf.WriteString("</lastmod>\n")
	}
	buf.WriteString("<changefreq>")
	buf.WriteString(frequency)
	buf.WriteString("</changefreq>\n<priority>")
	buf.WriteString(priority)
	buf.WriteString("</priority>\n</url>\n")
}

func (s *server) sitemap(w http.ResponseWriter, request *http.Request) {
	entries, _ := s.store.SitemapEntries(request.Context(), 5000)
	var body bytes.Buffer
	body.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	body.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	sitemapURL(&body, s.baseURL+"/", "", "daily", "1")
	sitemapURL(&body, s.baseURL+"/jobs", "", "daily", "0.95")
	sitemapURL(&body, s.baseURL+"/docs", "", "weekly", "0.8")
	sitemapURL(&body, s.baseURL+"/privacy", "", "yearly", "0.3")
	sitemapURL(&body, s.baseURL+"/vs/intern-insider", "", "monthly", "0.6")
	for _, entry := range entries {
		sitemapURL(&body, s.baseURL+entry.Path, entry.LastModified, "daily", "0.7")
	}
	body.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	_, _ = w.Write(body.Bytes())
}
