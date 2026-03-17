package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"alldev-gin-rpc/pkg/status_code"
)

func TestResponse_Struct(t *testing.T) {
	resp := Response{
		Code:      200,
		Msg:       "success",
		Data:      map[string]interface{}{"key": "value"},
		Timestamp: time.Now().Unix(),
		RequestID: "test-123",
	}

	if resp.Code != 200 {
		t.Errorf("Expected Code 200, got %d", resp.Code)
	}
	if resp.Msg != "success" {
		t.Errorf("Expected Msg 'success', got %s", resp.Msg)
	}
	if resp.Data == nil {
		t.Error("Expected Data to not be nil")
	}
	if resp.Timestamp == 0 {
		t.Error("Expected Timestamp to not be 0")
	}
	if resp.RequestID != "test-123" {
		t.Errorf("Expected RequestID 'test-123', got %s", resp.RequestID)
	}
}

func TestSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Success(c, map[string]interface{}{"message": "test data"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != int(status_code.Success) {
		t.Errorf("Expected status %d, got %d", int(status_code.Success), w.Code)
	}

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Code != int(status_code.Success) {
		t.Errorf("Expected response code %d, got %d", int(status_code.Success), response.Code)
	}
	if response.Msg != status_code.Success.Message() {
		t.Errorf("Expected message '%s', got '%s'", status_code.Success.Message(), response.Msg)
	}
	if response.Data == nil {
		t.Error("Expected data to not be nil")
	}
	if response.Timestamp == 0 {
		t.Error("Expected timestamp to not be 0")
	}
}

func TestSuccess_WithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request-123")
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		Success(c, "test data")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.RequestID != "test-request-123" {
		t.Errorf("Expected RequestID 'test-request-123', got '%s'", response.RequestID)
	}
}

func TestSuccess_WithNilData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Success(c, nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data != nil {
		t.Error("Expected data to be nil")
	}
}

func TestError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Error(c, "test error", map[string]interface{}{"field": "value"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != int(status_code.BadRequest) {
		t.Errorf("Expected status %d, got %d", int(status_code.BadRequest), w.Code)
	}

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Code != int(status_code.BadRequest) {
		t.Errorf("Expected response code %d, got %d", int(status_code.BadRequest), response.Code)
	}
	if response.Msg != "test error" {
		t.Errorf("Expected message 'test error', got '%s'", response.Msg)
	}
	if response.Data == nil {
		t.Error("Expected data to not be nil")
	}
	if response.Timestamp == 0 {
		t.Error("Expected timestamp to not be 0")
	}
}

func TestError_WithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "error-request-456")
		c.Next()
	})
	router.GET("/test", func(c *gin.Context) {
		Error(c, "test error", nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.RequestID != "error-request-456" {
		t.Errorf("Expected RequestID 'error-request-456', got '%s'", response.RequestID)
	}
}

func TestError_WithNilData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Error(c, "test error", nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data != nil {
		t.Error("Expected data to be nil")
	}
}

func TestConstants(t *testing.T) {
	if SuccessCode != status_code.Success {
		t.Errorf("Expected SuccessCode to be %v, got %v", status_code.Success, SuccessCode)
	}
	if ErrorCode != status_code.BadRequest {
		t.Errorf("Expected ErrorCode to be %v, got %v", status_code.BadRequest, ErrorCode)
	}
}

func TestResponse_JSON_Marshaling(t *testing.T) {
	resp := Response{
		Code:      200,
		Msg:       "success",
		Data:      map[string]string{"key": "value"},
		Timestamp: time.Now().Unix(),
		RequestID: "test-123",
	}

	// Test marshaling
	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Test unmarshaling
	var unmarshaled Response
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if unmarshaled.Code != resp.Code {
		t.Errorf("Expected Code %d, got %d", resp.Code, unmarshaled.Code)
	}
	if unmarshaled.Msg != resp.Msg {
		t.Errorf("Expected Msg %s, got %s", resp.Msg, unmarshaled.Msg)
	}
	if unmarshaled.RequestID != resp.RequestID {
		t.Errorf("Expected RequestID %s, got %s", resp.RequestID, unmarshaled.RequestID)
	}
}

func TestResponse_WithComplexData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	complexData := struct {
		Name    string   `json:"name"`
		Age     int      `json:"age"`
		Tags    []string `json:"tags"`
		Details map[string]interface{} `json:"details"`
	}{
		Name: "John Doe",
		Age:  30,
		Tags: []string{"developer", "golang"},
		Details: map[string]interface{}{
			"experience": "5 years",
			"skills":     []string{"Go", "Python", "JavaScript"},
		},
	}

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Success(c, complexData)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Data == nil {
		t.Fatal("Expected data to not be nil")
	}

	// Verify complex data structure
	dataBytes, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}

	var data struct {
		Name    string                 `json:"name"`
		Age     int                    `json:"age"`
		Tags    []string               `json:"tags"`
		Details map[string]interface{} `json:"details"`
	}

	if err := json.Unmarshal(dataBytes, &data); err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", data.Name)
	}
	if data.Age != 30 {
		t.Errorf("Expected age 30, got %d", data.Age)
	}
	if len(data.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(data.Tags))
	}
}

func TestResponse_EmptyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Error(c, "", nil)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Msg != "" {
		t.Errorf("Expected empty message, got '%s'", response.Msg)
	}
}

func TestResponse_TimestampAccuracy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	before := time.Now().Unix()
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Success(c, "test")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	after := time.Now().Unix()

	var response Response
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Timestamp < before || response.Timestamp > after {
		t.Errorf("Expected timestamp between %d and %d, got %d", before, after, response.Timestamp)
	}
}

func TestResponse_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		Success(c, map[string]int{"id": 123})
	})

	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var response Response
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}
			
			if response.Code != int(status_code.Success) {
				t.Errorf("Expected success code, got %d", response.Code)
			}
			
			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}
