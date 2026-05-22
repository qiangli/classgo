package handlers

import (
	"net/http"
	"strings"

	"classgo/internal/models"
)

// basePathFromReq returns the per-request URL prefix that the upstream
// proxy chain stripped before the request reached us. It populates the
// PageHead.BasePath so templates can render `<base href="{prefix}/">`
// and every relative URL in the page resolves back through the same
// proxy chain — making the app addressable identically at:
//
//   - http://localhost:8080/                       (direct)
//   - http://localhost:17777/class/                (outpost loopback)
//   - https://ai.dhnt.io/h/dragon/app/class/       (cloudbox tunnel)
//
// X-Forwarded-Prefix is set by every paired proxy (outpost stamps
// /app/<name>; cloudbox stamps /h/<host>/app/<name>). Empty when the
// request hits us directly with no proxy in front.
func basePathFromReq(r *http.Request) string {
	return strings.TrimRight(r.Header.Get("X-Forwarded-Prefix"), "/")
}

// pageHead is shorthand for the embedded PageHead value handlers use
// when constructing a *Data struct.
func pageHead(r *http.Request) models.PageHead {
	return models.PageHead{BasePath: basePathFromReq(r)}
}
