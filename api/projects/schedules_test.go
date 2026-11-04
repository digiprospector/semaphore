package projects

import (
	"net/http/httptest"
	"testing"

	"github.com/semaphoreui/semaphore/db"
	"github.com/stretchr/testify/assert"
)

func TestValidateOnBootSchedulePayload(t *testing.T) {
	tests := []struct {
		name          string
		delay         int
		valid         bool
		wantErrorText string
	}{
		{name: "zero delay", delay: 0, valid: true},
		{name: "positive delay", delay: 30, valid: true},
		{name: "negative delay", delay: -1, wantErrorText: "time_after_boot must be between 0 and 2147483647 seconds"},
		{name: "too large delay", delay: 2147483648, wantErrorText: "time_after_boot must be between 0 and 2147483647 seconds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := db.Schedule{
				Type:           db.ScheduleTypeOnBoot,
				CronFormat:     "* * * * *",
				TimeAfterBoot:  tt.delay,
				DeleteAfterRun: true,
			}
			w := httptest.NewRecorder()

			valid := validateSchedulePayload(&schedule, w)

			assert.Equal(t, tt.valid, valid)
			if tt.valid {
				assert.Empty(t, schedule.CronFormat)
				assert.Nil(t, schedule.RunAt)
				assert.False(t, schedule.DeleteAfterRun)
			} else {
				assert.Contains(t, w.Body.String(), tt.wantErrorText)
			}
		})
	}
}
