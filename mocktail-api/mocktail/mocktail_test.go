package mocktail

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"mocktail-api/database"

	"github.com/gofiber/fiber/v2"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&Api{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DBConn = db
}

func testApp() *fiber.App {
	app := fiber.New()
	app.All("/mocktail/*", MockApiHandler)
	return app
}

func rawJSON(s string) datatypes.JSON { return datatypes.JSON([]byte(s)) }

func insert(t *testing.T, a *Api) {
	t.Helper()
	a.Key = a.Method + a.Endpoint
	if err := database.DBConn.Create(a).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func get(t *testing.T, app *fiber.App, method, path string) (int, []byte) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil), -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestServeMock(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{Endpoint: "foo", Method: "GET", StatusCode: 200, Response: rawJSON(`{"ok":true}`)})

	status, body := get(t, app, "GET", "/mocktail/foo")
	if status != 200 {
		t.Fatalf("status = %d, want 200; body=%s", status, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["ok"] != true {
		t.Errorf("body = %s, want {\"ok\":true}", body)
	}
}

func TestServeNotFound(t *testing.T) {
	setupDB(t)
	app := testApp()
	status, _ := get(t, app, "GET", "/mocktail/nope")
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestServeCustomStatus(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{Endpoint: "boom", Method: "GET", StatusCode: 503, Response: rawJSON(`{"err":"x"}`)})

	status, _ := get(t, app, "GET", "/mocktail/boom")
	if status != 503 {
		t.Errorf("status = %d, want 503", status)
	}
}

func TestServe204NoBody(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{Endpoint: "empty", Method: "GET", StatusCode: 204, Response: rawJSON(`{"ignored":true}`)})

	status, body := get(t, app, "GET", "/mocktail/empty")
	if status != 204 {
		t.Fatalf("status = %d, want 204", status)
	}
	if len(body) != 0 {
		t.Errorf("204 body = %q, want empty", body)
	}
}

func TestServeCustomHeaders(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{
		Endpoint:   "hdr",
		Method:     "GET",
		StatusCode: 200,
		Response:   rawJSON(`{"ok":true}`),
		Headers:    rawJSON(`{"X-Total-Count":"42","Cache-Control":"no-cache"}`),
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/mocktail/hdr", nil), -1)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Total-Count"); got != "42" {
		t.Errorf("X-Total-Count = %q, want 42", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	// Content-Type still defaults to JSON when not overridden.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestServeContentTypeOverride(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{
		Endpoint:   "ct",
		Method:     "GET",
		StatusCode: 200,
		Response:   rawJSON(`{"error":"bad"}`),
		Headers:    rawJSON(`{"Content-Type":"application/problem+json"}`),
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/mocktail/ct", nil), -1)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json (user override must win)", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, body)
	}
	if out["error"] != "bad" {
		t.Errorf("body = %s, want {\"error\":\"bad\"}", body)
	}
}

func TestServeRequestRulesExactAbsentPresentOrderingAndFallback(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{Endpoint: "rules", Method: "GET", StatusCode: 200, Response: rawJSON(`{"case":"default"}`), Rules: rawJSON(`[
		{"query":{"mode":{"exact":"fast"}},"statusCode":201,"response":{"case":"exact"}},
		{"headers":{"X-Trace":{"present":false}},"statusCode":202,"response":{"case":"absent"}},
		{"headers":{"X-Trace":{"present":true}},"statusCode":203,"response":{"case":"present"}}
	]`)})
	status, body := get(t, app, "GET", "/mocktail/rules?mode=fast")
	if status != 201 || !strings.Contains(string(body), `"exact"`) {
		t.Fatalf("exact rule: status=%d body=%s", status, body)
	}
	status, body = get(t, app, "GET", "/mocktail/rules?mode=slow")
	if status != 202 || !strings.Contains(string(body), `"absent"`) {
		t.Fatalf("absent rule: status=%d body=%s", status, body)
	}
	status, body = getWithHeaders(t, app, "GET", "/mocktail/rules?mode=slow", map[string]string{"X-Trace": "1"})
	if status != 203 || !strings.Contains(string(body), `"present"`) {
		t.Fatalf("present rule: status=%d body=%s", status, body)
	}
	status, body = get(t, app, "GET", "/mocktail/rules?mode=other")
	if status != 202 {
		t.Fatalf("fallback after matching absent rule: status=%d body=%s", status, body)
	}
}

func getWithHeaders(t *testing.T, app *fiber.App, method, path string, headers map[string]string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestValidateRulesAcceptsStringExactMatcher(t *testing.T) {
	if err := ValidateRules([]byte(`[{"query":{"mode":"preview"},"statusCode":200,"response":{"ok":true}}]`)); err != nil {
		t.Fatalf("string exact matcher should be accepted: %v", err)
	}
}

func TestValidateRulesRejectsMalformedRules(t *testing.T) {
	if err := ValidateRules([]byte(`[{"query":{"x":{}},"statusCode":200,"response":{}}]`)); err == nil {
		t.Fatal("expected matcher validation error")
	}
	if err := ValidateRules([]byte(`[{"statusCode":200,"response":null}]`)); err == nil {
		t.Fatal("expected response validation error")
	}
	if err := ValidateRules([]byte(`[{"statusCode":99,"response":{}}]`)); err == nil {
		t.Fatal("expected status validation error")
	}
}

func TestServeRandomizeFixed(t *testing.T) {
	setupDB(t)
	app := testApp()
	insert(t, &Api{
		Endpoint:   "rnd",
		Method:     "GET",
		StatusCode: 200,
		Response:   rawJSON(`{"name":"orig"}`),
		Randomize:  rawJSON(`{"name":{"type":"fixed","value":"Z"}}`),
	})

	status, body := get(t, app, "GET", "/mocktail/rnd")
	if status != 200 {
		t.Fatalf("status = %d", status)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["name"] != "Z" {
		t.Errorf("name = %v, want Z (per-request randomize applied)", out["name"])
	}
}
