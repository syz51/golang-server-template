package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/syz51/golang-server-template/internal/config"
	"github.com/syz51/golang-server-template/internal/model"
	"github.com/syz51/golang-server-template/internal/service"
)

const validUserBody = `{"email":"user@example.com","first_name":"John","last_name":"Doe","age":25,"phone":"+1234567890"}`

func regressionAPI() (*echo.Echo, *Handler) {
	e := echo.New()
	h := New(&config.Config{App: config.AppConfig{Name: "test-service", Version: "test-version"}})
	e.GET("/health", h.Health)
	e.POST("/users", h.CreateUser)
	e.GET("/users", h.ListUsers)
	e.GET("/users/:id", h.GetUser)
	e.PUT("/users/:id", h.UpdateUser)
	e.DELETE("/users/:id", h.DeleteUser)
	return e, h
}

func perform(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestUserHTTPStatusesAndPhone(t *testing.T) {
	e, _ := regressionAPI()
	if rec := perform(e, http.MethodPost, "/users", validUserBody); rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	second := strings.Replace(validUserBody, "user@example.com", "other@example.com", 1)
	if rec := perform(e, http.MethodPost, "/users", second); rec.Code != 201 {
		t.Fatal(rec.Body)
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/users", validUserBody, 409},
		{http.MethodPut, "/users/2", `{"email":"user@example.com"}`, 409},
		{http.MethodGet, "/users/99", "", 404},
		{http.MethodPut, "/users/99", `{}`, 404},
		{http.MethodDelete, "/users/99", "", 404},
		{http.MethodGet, "/users/invalid", "", 400},
		{http.MethodPut, "/users/invalid", `{}`, 400},
		{http.MethodDelete, "/users/invalid", "", 400},
		{http.MethodPut, "/users/1", `{"age":0}`, 400},
		{http.MethodPut, "/users/1", `{"age":151}`, 400},
		{http.MethodPut, "/users/1", `{"email":""}`, 400},
		{http.MethodPut, "/users/1", `{"first_name":""}`, 400},
		{http.MethodPut, "/users/1", `{"status":""}`, 400},
		{http.MethodPut, "/users/1", `{"phone":"bad"}`, 400},
		{http.MethodPost, "/users", `{`, 400},
		{http.MethodPut, "/users/1", `{`, 400},
		{http.MethodPost, "/users", `{"age":"wrong type"}`, 400},
		{http.MethodPut, "/users/1", `{"age":"wrong type"}`, 400},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			rec := perform(e, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.status, rec.Body)
			}
		})
	}
	for _, tc := range []struct{ body, want string }{
		{`{}`, "+1234567890"}, {`{"phone":null}`, "+1234567890"}, {`{"phone":""}`, ""},
		{`{"phone":"+19876543210"}`, "+19876543210"},
	} {
		rec := perform(e, http.MethodPut, "/users/1", tc.body)
		if rec.Code != 200 {
			t.Fatalf("phone update: %d %s", rec.Code, rec.Body)
		}
		rec = perform(e, http.MethodGet, "/users/1", "")
		var user model.User
		if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}
		if user.Phone != tc.want {
			t.Fatalf("body=%s phone=%q want=%q", tc.body, user.Phone, tc.want)
		}
	}
	if rec := perform(e, http.MethodDelete, "/users/1", ""); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
}

func TestListUsersHTTPPagination(t *testing.T) {
	e, h := regressionAPI()
	for i := 0; i < 7; i++ {
		if _, err := h.userService.CreateUser(&model.CreateUserRequest{Email: fmt.Sprintf("%d@example.com", i)}); err != nil {
			t.Fatal(err)
		}
	}
	maxInt := int(^uint(0) >> 1)
	for _, query := range []string{"page=0", "page=-1", "page=abc", "page=", "page=1&page=2", "page=%zz", "per_page=%zz", "page=1;page=2", "page=1&page=%zz", "per_page=1&per_page=%zz", "per_page=1;per_page=2", "per_page=1&per_page=2", "per_page=0", "per_page=-1", "per_page=101", "per_page=abc", "per_page=", "page=99999999999999999999999999999999", fmt.Sprintf("page=%d&per_page=100", maxInt)} {
		rec := perform(e, http.MethodGet, "/users?"+query, "")
		if rec.Code != 400 {
			t.Errorf("%s: %d %s", query, rec.Code, rec.Body)
		}
	}
	for repeat := 0; repeat < 10; repeat++ {
		rec := perform(e, http.MethodGet, "/users?page=2&per_page=3", "")
		var list model.UserListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || list.Meta.TotalPages != 3 || len(list.Users) != 3 {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		for i, user := range list.Users {
			if user.ID != i+4 {
				t.Fatalf("unstable order: %s", rec.Body)
			}
		}
	}
	for _, query := range []string{"page=999&per_page=100", fmt.Sprintf("page=%d&per_page=1", maxInt)} {
		rec := perform(e, http.MethodGet, "/users?"+query, "")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"users":[]`) {
			t.Fatalf("out-of-range: %d %s", rec.Code, rec.Body)
		}
	}
	rec := perform(e, http.MethodGet, "/users", "")
	var list model.UserListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Meta.Page != 1 || list.Meta.PerPage != 10 {
		t.Fatalf("defaults: %+v", list.Meta)
	}
}

func TestBindingTransportStatuses(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, unknownLength := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/chunked=%t", method, unknownLength), func(t *testing.T) {
				e, _ := regressionAPI()
				e.Use(echomiddleware.BodyLimit(128))
				server := httptest.NewServer(e)
				defer server.Close()
				path := "/users"
				if method == http.MethodPut {
					path += "/1"
				}
				body := strings.Replace(validUserBody, "John", strings.Repeat("J", 1024), 1)
				var reader io.Reader = strings.NewReader(body)
				if unknownLength {
					reader = struct{ io.Reader }{reader}
				}
				req, err := http.NewRequest(method, server.URL+path, reader)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				if unknownLength && req.ContentLength != 0 {
					t.Fatalf("expected unknown length, got %d", req.ContentLength)
				}
				res, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 413 {
					body, _ := io.ReadAll(res.Body)
					t.Fatalf("status=%d body=%s", res.StatusCode, body)
				}
			})
		}
		t.Run(method+"/unsupported", func(t *testing.T) {
			e, _ := regressionAPI()
			req := httptest.NewRequest(method, "/users", strings.NewReader("payload"))
			if method == http.MethodPut {
				req = httptest.NewRequest(method, "/users/1", strings.NewReader("payload"))
			}
			req.Header.Set(echo.HeaderContentType, "text/plain")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != 415 {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
			}
		})
	}
	// net/http limiters wrap a non-HTTP error; preserve that limit too.
	e, _ := regressionAPI()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 32)
			return next(c)
		}
	})
	if rec := perform(e, http.MethodPost, "/users", validUserBody); rec.Code != 413 {
		t.Fatalf("MaxBytesReader: %d %s", rec.Code, rec.Body)
	}
}

func TestHealthIsLivenessOnly(t *testing.T) {
	e, _ := regressionAPI()
	before := time.Now()
	rec := perform(e, http.MethodGet, "/health", "")
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(body) != 4 {
		t.Fatalf("health response: %d %s", rec.Code, rec.Body)
	}
	for _, key := range []string{"status", "service", "version", "timestamp"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	var health model.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if health.Status != "ok" || health.Service != "test-service" || health.Version != "test-version" || health.Timestamp.Before(before) || health.Timestamp.After(time.Now()) {
		t.Fatalf("health: %+v", health)
	}
}

func TestWrappedServiceErrorStatuses(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{service.ErrUserNotFound, 404}, {service.ErrEmailExists, 409}, {service.ErrInvalidPagination, 400}, {errors.New("unexpected"), 500},
	} {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
		if err := serviceError(c, fmt.Errorf("context: %w", tc.err)); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tc.status {
			t.Errorf("%v: status %d", tc.err, rec.Code)
		}
	}
}

func TestConcurrentHTTPGetPutSnapshots(t *testing.T) {
	e, h := regressionAPI()
	_, err := h.userService.CreateUser(&model.CreateUserRequest{Email: "race@example.com", FirstName: "Age25", LastName: "Doe", Age: 25})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(e)
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Second
	start := make(chan struct{})
	failures := make(chan error, 8)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for i := 0; i < 50; i++ {
				method := http.MethodGet
				var body io.Reader
				if worker%2 == 0 {
					method = http.MethodPut
					age := 1 + (worker*50+i)%150
					body = strings.NewReader(fmt.Sprintf(`{"first_name":"Age%d","age":%d}`, age, age))
				}
				req, err := http.NewRequest(method, server.URL+"/users/1", body)
				if err != nil {
					failures <- err
					return
				}
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				res, err := client.Do(req)
				if err != nil {
					failures <- err
					return
				}
				var user model.User
				err = json.NewDecoder(res.Body).Decode(&user)
				res.Body.Close()
				if err != nil {
					failures <- err
					return
				}
				if res.StatusCode != 200 || user.FirstName != "Age"+strconv.Itoa(user.Age) {
					failures <- fmt.Errorf("inconsistent snapshot: status=%d user=%+v", res.StatusCode, user)
					return
				}
			}
		}(worker)
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
