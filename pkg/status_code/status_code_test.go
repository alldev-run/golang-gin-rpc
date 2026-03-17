package status_code

import (
	"fmt"
	"testing"
)

func TestStatusCode_Values(t *testing.T) {
	tests := []struct {
		name     string
		status   StatusCode
		expected int
	}{
		{"Success", Success, 200},
		{"Internal", Internal, 500},
		{"BadRequest", BadRequest, 400},
		{"NotFound", NotFound, 404},
		{"Fail", Fail, 499},
		{"Error", Error, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.status) != tt.expected {
				t.Errorf("StatusCode %s = %d, want %d", tt.name, int(tt.status), tt.expected)
			}
		})
	}
}

func TestStatusCode_Message(t *testing.T) {
	tests := []struct {
		name     string
		status   StatusCode
		expected string
	}{
		{"Success", Success, "success"},
		{"Internal", Internal, "internal error"},
		{"BadRequest", BadRequest, "bad request"},
		{"NotFound", NotFound, "not found"},
		{"Fail", Fail, "fail"},
		{"Error", Error, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if msg := tt.status.Message(); msg != tt.expected {
				t.Errorf("StatusCode %s.Message() = %s, want %s", tt.name, msg, tt.expected)
			}
		})
	}
}

func TestStatusCode_UnknownMessage(t *testing.T) {
	unknownStatus := StatusCode(999)
	expected := "unknown error"
	
	if msg := unknownStatus.Message(); msg != expected {
		t.Errorf("Unknown status code message = %s, want %s", msg, expected)
	}
}

func TestStatusCode_ZeroValue(t *testing.T) {
	var zeroStatus StatusCode
	expected := "unknown error"
	
	if int(zeroStatus) != 0 {
		t.Errorf("Zero status code = %d, want 0", int(zeroStatus))
	}
	
	if msg := zeroStatus.Message(); msg != expected {
		t.Errorf("Zero status code message = %s, want %s", msg, expected)
	}
}

func TestStatusCode_NegativeValues(t *testing.T) {
	negativeStatus := StatusCode(-100)
	expected := "unknown error"
	
	if msg := negativeStatus.Message(); msg != expected {
		t.Errorf("Negative status code message = %s, want %s", msg, expected)
	}
}

func TestStatusCode_LargeValues(t *testing.T) {
	largeStatus := StatusCode(1000)
	expected := "unknown error"
	
	if msg := largeStatus.Message(); msg != expected {
		t.Errorf("Large status code message = %s, want %s", msg, expected)
	}
}

func TestStatusCode_ConstantComparisons(t *testing.T) {
	// Test that constants can be compared
	if Success != 200 {
		t.Error("Success constant should equal 200")
	}
	
	if Internal != 500 {
		t.Error("Internal constant should equal 500")
	}
	
	if BadRequest != 400 {
		t.Error("BadRequest constant should equal 400")
	}
	
	if NotFound != 404 {
		t.Error("NotFound constant should equal 404")
	}
	
	if Fail != 499 {
		t.Error("Fail constant should equal 499")
	}
	
	if Error != -1 {
		t.Error("Error constant should equal -1")
	}
}

func TestStatusCode_TypeConversion(t *testing.T) {
	// Test int to StatusCode conversion
	status := StatusCode(200)
	if status != Success {
		t.Error("StatusCode(200) should equal Success")
	}
	
	// Test StatusCode to int conversion
	if int(Success) != 200 {
		t.Error("int(Success) should equal 200")
	}
	
	// Test string conversion
	if Success.Message() != "success" {
		t.Error("Success.Message() should return 'success'")
	}
}

func TestStatusCode_EdgeCases(t *testing.T) {
	// Test status codes around the defined constants
	testCases := []struct {
		status   StatusCode
		expected string
	}{
		{199, "unknown error"}, // Just before Success
		{200, "success"},      // Success
		{201, "unknown error"}, // Just after Success
		{399, "unknown error"}, // Just before BadRequest
		{400, "bad request"},   // BadRequest
		{401, "unknown error"}, // Just after BadRequest
		{403, "unknown error"}, // Another common HTTP status
		{404, "not found"},     // NotFound
		{405, "unknown error"}, // Just after NotFound
		{498, "unknown error"}, // Just before Fail
		{499, "fail"},          // Fail
		{500, "internal error"}, // Internal
		{501, "unknown error"}, // Just after Internal
		{-2, "unknown error"},  // Negative
		{-1, "error"},          // Error
		{0, "unknown error"},   // Zero
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d", tc.status), func(t *testing.T) {
			if msg := tc.status.Message(); msg != tc.expected {
				t.Errorf("StatusCode %d.Message() = %s, want %s", int(tc.status), msg, tc.expected)
			}
		})
	}
}

func TestStatusCode_ConcurrentAccess(t *testing.T) {
	// Test that Message() can be called concurrently without race conditions
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			// Test different status codes
			statuses := []StatusCode{Success, Internal, BadRequest, NotFound, Fail, Error}
			for _, status := range statuses {
				msg := status.Message()
				if msg == "" {
					t.Errorf("Empty message for status %d in goroutine %d", int(status), id)
				}
			}
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStatusCode_MessageConsistency(t *testing.T) {
	// Test that Message() returns consistent results
	status := Success
	
	// Call Message() multiple times
	msg1 := status.Message()
	msg2 := status.Message()
	msg3 := status.Message()
	
	if msg1 != msg2 || msg2 != msg3 {
		t.Error("Message() should return consistent results")
	}
	
	expected := "success"
	if msg1 != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, msg1)
	}
}

func TestStatusCode_AllDefinedConstants(t *testing.T) {
	// Ensure all defined constants have messages
	constants := []StatusCode{Success, Internal, BadRequest, NotFound, Fail, Error}
	
	for _, status := range constants {
		msg := status.Message()
		if msg == "" {
			t.Errorf("Status code %d should have a message", int(status))
		}
		if msg == "unknown error" {
			t.Errorf("Status code %d should not have unknown error message", int(status))
		}
	}
}
