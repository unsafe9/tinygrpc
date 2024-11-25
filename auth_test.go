package tinygrpc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestMultiAuthProvider_VerifyAuth(t *testing.T) {
	var (
		succeed AuthProviderFunc = func(c *CallContext) (context.Context, error) {
			passedCount, _ := c.ctx.Value("passedCount").(int)
			passedCount++
			return context.WithValue(c.ctx, "passedCount", passedCount), nil
		}
		fail AuthProviderFunc = func(c *CallContext) (context.Context, error) {
			return c.ctx, status.Error(codes.Unauthenticated, "authentication failed")
		}
	)

	tests := []struct {
		name                 string
		providers            []AuthProvider
		allowUnauthenticated bool
		evaluateAll          bool
		requireAll           bool
		expectedErrorCode    codes.Code
		expectedPassedCount  int
	}{
		{
			name:                 "all providers fail, allowUnauthenticated true",
			providers:            []AuthProvider{fail, fail},
			allowUnauthenticated: true,
			evaluateAll:          false,
			requireAll:           false,
			expectedErrorCode:    codes.OK,
			expectedPassedCount:  0,
		},
		{
			name:                 "all providers fail, allowUnauthenticated false",
			providers:            []AuthProvider{fail, fail},
			allowUnauthenticated: false,
			evaluateAll:          false,
			requireAll:           false,
			expectedErrorCode:    codes.Unauthenticated,
			expectedPassedCount:  0,
		},
		{
			name:                 "one provider succeeds, requireAll false",
			providers:            []AuthProvider{fail, succeed},
			allowUnauthenticated: false,
			evaluateAll:          false,
			requireAll:           false,
			expectedErrorCode:    codes.OK,
			expectedPassedCount:  1,
		},
		{
			name:                 "one provider succeeds, requireAll true",
			providers:            []AuthProvider{succeed, fail},
			allowUnauthenticated: false,
			evaluateAll:          false,
			requireAll:           true,
			expectedErrorCode:    codes.Unauthenticated,
			expectedPassedCount:  1,
		},
		{
			name:                 "all providers succeed, evaluateAll false",
			providers:            []AuthProvider{succeed, succeed},
			allowUnauthenticated: false,
			evaluateAll:          false,
			requireAll:           false,
			expectedErrorCode:    codes.OK,
			expectedPassedCount:  1,
		},
		{
			name:                 "one provider succeeds, evaluateAll true",
			providers:            []AuthProvider{fail, succeed},
			allowUnauthenticated: false,
			evaluateAll:          true,
			requireAll:           false,
			expectedErrorCode:    codes.OK,
			expectedPassedCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multiAuthProvider := &MultiAuthProvider{
				Providers:            tt.providers,
				AllowUnauthenticated: tt.allowUnauthenticated,
				EvaluateAll:          tt.evaluateAll,
				RequireAll:           tt.requireAll,
			}

			ctx := context.Background()
			callContext := &CallContext{
				ctx: ctx,
			}

			ctx, err := multiAuthProvider.VerifyAuth(callContext)
			if err != nil {
				if status.Code(err) != tt.expectedErrorCode {
					t.Errorf("expected error code %v, got %v", tt.expectedErrorCode, status.Code(err))
				}
			} else {
				if tt.expectedErrorCode != codes.OK {
					t.Errorf("expected error code %v, got %v", tt.expectedErrorCode, codes.OK)
				}
			}

			var passedCount int
			passedCount, _ = ctx.Value("passedCount").(int)
			if passedCount != tt.expectedPassedCount {
				t.Errorf("expected passed count %v, got %v", tt.expectedPassedCount, passedCount)
			}
		})
	}
}
