//go:build integration

package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F11 — границы доступа. Ист.: backend/README «Guest access», SERVICES.md «Internal», gateway/handler.go, httpx.

// F11-01 + F11-07 Чужая загрузка невидима отовсюду, и остаётся такой после «Найти полный текст».
func TestF11_ForeignUploadIsInvisible(t *testing.T) {
	owner := user(t, "f11-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, 200, owner.WaitForPdfReady(private.ID, 90*time.Second).Status)

	b := user(t, "f11-b")
	probe := func() {
		t.Helper()
		id := private.ID
		cases := []struct {
			method, path string
			body         any
		}{
			{"GET", "/papers/" + id, nil},
			{"GET", "/papers/" + id + "/pdf-url", nil},
			{"GET", "/papers/" + id + "/pdf", nil},
			{"GET", "/papers/" + id + "/annotations", nil},
			{"POST", "/papers/" + id + "/annotations", map[string]any{"page": 1, "selected_text": "x", "note": ""}},
			{"GET", "/papers/" + id + "/chat/messages", nil},
			{"POST", "/papers/" + id + "/chat", map[string]any{"message": "hi"}},
			{"POST", "/papers/" + id + "/explain", map[string]any{"text": "hi"}},
			{"GET", "/papers/" + id + "/summary", nil},
			{"POST", "/papers/" + id + "/summary", nil},
			{"POST", "/papers/" + id + "/translate", map[string]any{"text": "hi", "target_lang": "en"}},
			{"POST", "/papers/" + id + "/find-fulltext", nil},
			{"POST", "/papers/" + id + "/retry-pdf", nil},
			{"POST", "/library/" + id, map[string]any{}},
			{"GET", "/library/" + id, nil},
		}
		for _, c := range cases {
			resp := b.Do(c.method, c.path, c.body)
			require.Equalf(t, 404, resp.Status, "%s %s → %s", c.method, c.path, resp)
		}
	}
	probe()

	_, resp = owner.FindFullText(private.ID)
	require.Contains(t, []int{200, 422}, resp.Status, resp)
	probe()
}

// F11-02 Заголовок X-User-Id от вошедшего пользователя не подменяет личность.
func TestF11_UserIDHeaderIsIgnored(t *testing.T) {
	a := user(t, "f11-hdr-a")
	b := user(t, "f11-hdr-b")
	aID := a.Get("/auth/me").Map(t)["id"].(string)
	paper := publicPaper(t, a.Session, "2301.00001")
	_, resp := a.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)

	me := b.Get("/auth/me", "X-User-Id", aID)
	require.Equal(t, 200, me.Status, me)
	require.Equal(t, b.Email, me.Map(t)["email"])
	lib := b.Get("/library", "X-User-Id", aID)
	require.Equal(t, 200, lib.Status, lib)
	require.EqualValues(t, 0, lib.Map(t)["total"], "B sees B's empty library, not A's")
}

// F11-03 Внутренний маршрут каталога снаружи недоступен.
func TestF11_InternalRoutesAreNotExposed(t *testing.T) {
	u := user(t, "f11-internal")
	paper := publicPaper(t, u.Session, "2301.00001")
	path := "/internal/papers/" + paper.ID + "/access"
	require.Equal(t, 401, guest(t).Get(path).Status)
	resp := u.Get(path)
	require.Equal(t, 404, resp.Status, resp)
	resp = u.Get(path, "X-Internal-Token", "test-internal-token")
	require.Equal(t, 404, resp.Status, "even with the internal token: %s", resp)
}

// forgedJWT signs {sub,type} with the given secret the way the backend would.
func forgedJWT(t *testing.T, secret, sub, typ string) string {
	t.Helper()
	enc := func(v any) string {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	head := enc(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims := enc(map[string]any{"sub": sub, "type": typ, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix()})
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(head + "." + claims))
	return head + "." + claims + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// F11-04 Поддельные и неподходящие токены отклоняются.
func TestF11_ForgedTokens(t *testing.T) {
	u := user(t, "f11-tokens")
	id := u.Get("/auth/me").Map(t)["id"].(string)
	cases := map[string]string{
		"wrong secret":               forgedJWT(t, "not-the-stand-secret", id, "access"),
		"refresh token as access":    forgedJWT(t, "test-stand-secret", id, "refresh"),
		"unknown user, right secret": forgedJWT(t, "test-stand-secret", "00000000-0000-0000-0000-000000000001", "access"),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			s := guest(t)
			s.Token = token
			resp := s.Get("/library")
			require.Equal(t, 401, resp.Status, resp)
			require.NotEmpty(t, resp.Detail())
		})
	}
	t.Run("empty bearer", func(t *testing.T) {
		resp := guest(t).Get("/library", "Authorization", "Bearer ")
		require.Equal(t, 401, resp.Status, resp)
	})
}

// F11-05 Лимиты и форма JSON-тела.
func TestF11_JSONBodyLimits(t *testing.T) {
	u := user(t, "f11-json")
	huge := `{"name":"` + strings.Repeat("x", 1<<20) + `"}`
	require.Equal(t, 400, u.Post("/library/folders", huge).Status, "1 MB+ body")
	require.Equal(t, 400, u.Post("/library/folders", `[{"name":"x"}]`).Status, "array instead of object")
	require.Equal(t, 400, u.Post("/library/folders", `{"name":"x"} {"name":"y"}`).Status, "two objects")
	require.Equal(t, 400, u.Post("/library/folders", map[string]any{"name": "x", "icon": "y"}).Status, "unknown field")
	require.Equal(t, 400, u.Post("/auth/login", map[string]any{"email": u.Email, "password": u.Password, "remember": true}).Status, "unknown field on auth too")
}

// F11-06 CORS: только известный фронт получает разрешение с credentials.
func TestF11_CORS(t *testing.T) {
	g := guest(t)
	preflight := func(origin string) client.Response {
		return g.Do(http.MethodOptions, "/library", nil, "Origin", origin, "Access-Control-Request-Method", "PATCH", "Access-Control-Request-Headers", "Authorization, Content-Type")
	}
	ok := preflight("http://localhost:5173")
	require.Contains(t, []int{200, 204}, ok.Status, ok)
	require.Equal(t, "http://localhost:5173", ok.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", ok.Header.Get("Access-Control-Allow-Credentials"))
	require.Contains(t, ok.Header.Get("Access-Control-Allow-Methods"), "PATCH")
	require.Contains(t, ok.Header.Get("Access-Control-Allow-Headers"), "Authorization")

	evil := preflight("http://evil.example")
	require.Empty(t, evil.Header.Get("Access-Control-Allow-Origin"), "an unknown origin gets no CORS grant: %s", evil)
}
