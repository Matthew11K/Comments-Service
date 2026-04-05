package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Matthew11K/Comments-Service/internal/testutil/postgresitest"
)

var suite *postgresitest.Suite

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

	var err error
	suite, err = postgresitest.Start(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	exitCode := m.Run()

	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
	if err := suite.Close(closeCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		if exitCode == 0 {
			exitCode = 1
		}
	}
	closeCancel()

	os.Exit(exitCode)
}
