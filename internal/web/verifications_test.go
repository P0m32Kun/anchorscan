package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/P0m32Kun/anchorscan/internal/store"
)

func setupProjectWithVerification(t *testing.T, opts ServerOptions) (http.Handler, string, string, string) {
	t.Helper()
	handler, err := NewServer(opts)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	closeServer(t, handler)

	// Direct store access to bootstrap the project and verification.
	// The server opened its own Store, so open a separate one on the same DB.
	s, err := store.Open(opts.DBPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	if err := s.SaveProject(store.Project{ID: "p1", Name: "Task", CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0)}); err != nil {
		t.Fatalf("SaveProject returned error: %v", err)
	}
	if err := s.CreateDefaultProjectZones("p1"); err != nil {
		t.Fatalf("CreateDefaultProjectZones returned error: %v", err)
	}
	v := store.Verification{
		ID: "v1", ProjectID: "p1", ZoneID: "I", Outcome: "confirmed", Title: "Redis 默认登录",
		CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(1, 0),
	}
	if err := s.CreateVerification(v, nil, nil); err != nil {
		t.Fatalf("CreateVerification returned error: %v", err)
	}
	return handler, "p1", "v1", opts.DBPath
}

func generateTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 3))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode returned error: %v", err)
	}
	return buf.Bytes()
}

func uploadEvidence(t *testing.T, handler http.Handler, url string, data []byte, caption string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "screenshot.png")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	_ = writer.WriteField("caption", caption)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func TestEvidenceUploadStoresFileAndReturnsMetadata(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, projectID, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := uploadEvidence(t, handler, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", generateTestPNG(t), "confirmed on console")
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	var ev store.VerificationEvidence
	if err := json.Unmarshal(res.Body.Bytes(), &ev); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if ev.MediaType != "image/png" || ev.Caption != "confirmed on console" {
		t.Fatalf("unexpected evidence metadata: %#v", ev)
	}

	// Verify the file exists under the managed project directory.
	absPath := filepath.Join(dir, "projects", projectID, ev.RelativePath)
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("evidence file missing: %v", err)
	}
}

func TestEvidenceUploadRejectsNonImage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, projectID, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := uploadEvidence(t, handler, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", []byte("not an image"), "bad")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", res.Code, res.Body.String())
	}
}

func TestEvidenceUploadRejectsWrongProject(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, _, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := uploadEvidence(t, handler, "/projects/p2/verifications/"+verificationID+"/evidence", generateTestPNG(t), "x")
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", res.Code, res.Body.String())
	}
}

func TestEvidenceListAndServe(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, projectID, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := uploadEvidence(t, handler, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", generateTestPNG(t), "screenshot")
	if res.Code != http.StatusCreated {
		t.Fatalf("upload returned %d: %s", res.Code, res.Body.String())
	}
	var ev store.VerificationEvidence
	if err := json.Unmarshal(res.Body.Bytes(), &ev); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	listRes := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", nil)
	handler.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listRes.Code, listRes.Body.String())
	}
	var list []store.VerificationEvidence
	if err := json.Unmarshal(listRes.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != ev.ID {
		t.Fatalf("unexpected list: %#v", list)
	}

	serveRes := httptest.NewRecorder()
	serveReq := httptest.NewRequest(http.MethodGet, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence/"+ev.ID, nil)
	handler.ServeHTTP(serveRes, serveReq)
	if serveRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", serveRes.Code, serveRes.Body.String())
	}
	if serveRes.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected image/png content type, got %s", serveRes.Header().Get("Content-Type"))
	}
	got, _ := io.ReadAll(serveRes.Body)
	if !bytes.Equal(got, generateTestPNG(t)) {
		t.Fatalf("served file does not match uploaded bytes")
	}
}

func TestEvidenceUpdateCaptionAndDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, projectID, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := uploadEvidence(t, handler, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", generateTestPNG(t), "before")
	if res.Code != http.StatusCreated {
		t.Fatalf("upload returned %d: %s", res.Code, res.Body.String())
	}
	var ev store.VerificationEvidence
	if err := json.Unmarshal(res.Body.Bytes(), &ev); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	updateRes := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence/"+ev.ID, nil)
	updateReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updateReq.Form = map[string][]string{"caption": {"after"}}
	handler.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("update returned %d: %s", updateRes.Code, updateRes.Body.String())
	}
	var updated store.VerificationEvidence
	if err := json.Unmarshal(updateRes.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if updated.Caption != "after" {
		t.Fatalf("caption not updated: %s", updated.Caption)
	}

	deleteRes := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodPost, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence/"+ev.ID, nil)
	deleteReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	deleteReq.Form = map[string][]string{"_method": {"delete"}}
	handler.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete returned %d: %s", deleteRes.Code, deleteRes.Body.String())
	}

	s, _ := store.Open(dbPath)
	defer s.Close()
	list, _ := s.ListVerificationEvidence(verificationID)
	if len(list) != 0 {
		t.Fatalf("expected evidence to be deleted, got %d rows", len(list))
	}
}

func TestEvidenceListRejectsWrongProject(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, _, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/projects/p2/verifications/"+verificationID+"/evidence", nil)
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", res.Code, res.Body.String())
	}
}

func TestCreateVerificationEndpoint(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, _, _, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	payload := verificationCreateRequest{
		ZoneID:  "I",
		Outcome: "confirmed",
		Title:   "Created via API",
	}
	body, _ := json.Marshal(payload)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/p1/verifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	var v store.Verification
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if v.Title != "Created via API" || v.ProjectID != "p1" {
		t.Fatalf("unexpected verification: %#v", v)
	}
}

func TestCreateVerificationConfirmedAutoIncluded(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, _, _, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	payload := verificationCreateRequest{
		ZoneID:   "I",
		Outcome:  "confirmed",
		Title:    "Auto included",
		Included: false,
	}
	body, _ := json.Marshal(payload)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/p1/verifications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}
	var v store.Verification
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if !v.Included {
		t.Fatalf("expected confirmed verification to be auto-included, got Included=%v", v.Included)
	}
}

func TestUpdateVerificationConfirmedAutoIncludedWithoutEvidenceFails(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, _, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})

	payload := verificationUpdateRequest{ZoneID: "I", Outcome: "confirmed", Title: "Updated", Included: false}
	body, _ := json.Marshal(payload)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/p1/verifications/"+verificationID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for confirmed without evidence, got %d: %s", res.Code, res.Body.String())
	}
}

func TestUpdateVerificationConfirmedAutoIncludedWithEvidenceSucceeds(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "scan.db")
	handler, projectID, verificationID, _ := setupProjectWithVerification(t, ServerOptions{ConfigPath: filepath.Join(dir, "config.yaml"), DBPath: dbPath})
	uploadEvidence(t, handler, "/projects/"+projectID+"/verifications/"+verificationID+"/evidence", generateTestPNG(t), "evidence")

	payload := verificationUpdateRequest{ZoneID: "I", Outcome: "confirmed", Title: "Updated", Included: false}
	body, _ := json.Marshal(payload)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/projects/p1/verifications/"+verificationID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var v store.Verification
	if err := json.Unmarshal(res.Body.Bytes(), &v); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}
	if !v.Included {
		t.Fatalf("expected confirmed with evidence to be auto-included, got Included=%v", v.Included)
	}
}
