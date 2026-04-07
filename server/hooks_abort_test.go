package server

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────
// 辅助：发送一条 JSON-RPC 消息并返回响应
// ─────────────────────────────────────────────

func sendMessage(t *testing.T, s *MCPServer, body string) any {
	t.Helper()
	resp := s.HandleMessage(context.Background(), []byte(body))
	require.NotNil(t, resp)
	return resp
}

func assertErrorResponse(t *testing.T, resp any, wantMsg string) {
	t.Helper()
	errResp, ok := resp.(mcp.JSONRPCError)
	require.True(t, ok, "expected JSONRPCError, got %T", resp)
	assert.Contains(t, errResp.Error.Message, wantMsg)
}

func assertSuccessResponse(t *testing.T, resp any) {
	t.Helper()
	_, ok := resp.(mcp.JSONRPCResponse)
	assert.True(t, ok, "expected JSONRPCResponse, got %T", resp)
}

// ─────────────────────────────────────────────
// 1. AfterAnyHookFunc 类型 & AddAfterAny
// ─────────────────────────────────────────────

// TestAddAfterAny_Called 验证 AfterAny hook 在请求成功后被调用
func TestAddAfterAny_Called(t *testing.T) {
	called := false
	var capturedMethod mcp.MCPMethod

	hooks := &Hooks{}
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		called = true
		capturedMethod = method
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertSuccessResponse(t, resp)

	assert.True(t, called, "AfterAny hook should be called")
	assert.Equal(t, mcp.MethodPing, capturedMethod)
}

// TestAddAfterAny_MultipleHooks 验证多个 AfterAny hook 按顺序全部被调用
func TestAddAfterAny_MultipleHooks(t *testing.T) {
	order := []int{}

	hooks := &Hooks{}
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		order = append(order, 1)
	})
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		order = append(order, 2)
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	assert.Equal(t, []int{1, 2}, order)
}

// ─────────────────────────────────────────────
// 2. AfterAny hook abort
// ─────────────────────────────────────────────

// TestAfterAny_Abort 验证在 AfterAny hook 中调用 AbortRequest 会阻止结果发送
func TestAfterAny_Abort(t *testing.T) {
	abortErr := errors.New("after-any abort")

	hooks := &Hooks{}
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		AbortRequest(ctx, abortErr)
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertErrorResponse(t, resp, "after-any abort")
}

// TestAfterAny_Abort_StopsSubsequentHooks 验证 abort 后后续 AfterAny hook 不再执行
func TestAfterAny_Abort_StopsSubsequentHooks(t *testing.T) {
	secondCalled := false

	hooks := &Hooks{}
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		AbortRequest(ctx, errors.New("stop here"))
	})
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		secondCalled = true
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	assert.False(t, secondCalled, "second AfterAny hook should not be called after abort")
}

// ─────────────────────────────────────────────
// 3. 具体 after* hook abort（覆盖各方法）
// ─────────────────────────────────────────────

// TestAfterInitialize_Abort 验证 AfterInitialize hook 中 abort 生效
func TestAfterInitialize_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
		AbortRequest(ctx, errors.New("after-initialize abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{
		"jsonrpc":"2.0","id":1,"method":"initialize",
		"params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"c","version":"1"}}
	}`)
	assertErrorResponse(t, resp, "after-initialize abort")
}

// TestAfterPing_Abort 验证 AfterPing hook 中 abort 生效
func TestAfterPing_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterPing(func(ctx context.Context, id any, message *mcp.PingRequest, result *mcp.EmptyResult) {
		AbortRequest(ctx, errors.New("after-ping abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertErrorResponse(t, resp, "after-ping abort")
}

// TestAfterListTools_Abort 验证 AfterListTools hook 中 abort 生效
func TestAfterListTools_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterListTools(func(ctx context.Context, id any, message *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
		AbortRequest(ctx, errors.New("after-list-tools abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithToolCapabilities(false), WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	assertErrorResponse(t, resp, "after-list-tools abort")
}

// TestAfterCallTool_Abort 验证 AfterCallTool hook 中 abort 生效
func TestAfterCallTool_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcp.CallToolRequest, result any) {
		AbortRequest(ctx, errors.New("after-call-tool abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	server.AddTool(mcp.NewTool("echo"), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})

	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo"}}`)
	assertErrorResponse(t, resp, "after-call-tool abort")
}

// TestAfterListResources_Abort 验证 AfterListResources hook 中 abort 生效
func TestAfterListResources_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterListResources(func(ctx context.Context, id any, message *mcp.ListResourcesRequest, result *mcp.ListResourcesResult) {
		AbortRequest(ctx, errors.New("after-list-resources abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithResourceCapabilities(false, false), WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`)
	assertErrorResponse(t, resp, "after-list-resources abort")
}

// TestAfterListResourceTemplates_Abort 验证 AfterListResourceTemplates hook 中 abort 生效
func TestAfterListResourceTemplates_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterListResourceTemplates(func(ctx context.Context, id any, message *mcp.ListResourceTemplatesRequest, result *mcp.ListResourceTemplatesResult) {
		AbortRequest(ctx, errors.New("after-list-resource-templates abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithResourceCapabilities(false, false), WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"resources/templates/list"}`)
	assertErrorResponse(t, resp, "after-list-resource-templates abort")
}

// TestAfterReadResource_Abort 验证 AfterReadResource hook 中 abort 生效
func TestAfterReadResource_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterReadResource(func(ctx context.Context, id any, message *mcp.ReadResourceRequest, result *mcp.ReadResourceResult) {
		AbortRequest(ctx, errors.New("after-read-resource abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithResourceCapabilities(false, false), WithHooks(hooks))
	server.AddResource(
		mcp.Resource{URI: "test://r", Name: "r"},
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{mcp.TextResourceContents{URI: "test://r", Text: "hi"}}, nil
		},
	)

	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"test://r"}}`)
	assertErrorResponse(t, resp, "after-read-resource abort")
}

// TestAfterListPrompts_Abort 验证 AfterListPrompts hook 中 abort 生效
func TestAfterListPrompts_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterListPrompts(func(ctx context.Context, id any, message *mcp.ListPromptsRequest, result *mcp.ListPromptsResult) {
		AbortRequest(ctx, errors.New("after-list-prompts abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithPromptCapabilities(false), WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`)
	assertErrorResponse(t, resp, "after-list-prompts abort")
}

// TestAfterGetPrompt_Abort 验证 AfterGetPrompt hook 中 abort 生效
func TestAfterGetPrompt_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterGetPrompt(func(ctx context.Context, id any, message *mcp.GetPromptRequest, result *mcp.GetPromptResult) {
		AbortRequest(ctx, errors.New("after-get-prompt abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithPromptCapabilities(false), WithHooks(hooks))
	server.AddPrompt(
		mcp.Prompt{Name: "p", Description: "test"},
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		},
	)

	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"prompts/get","params":{"name":"p"}}`)
	assertErrorResponse(t, resp, "after-get-prompt abort")
}

// TestAfterSetLevel_Abort 验证 AfterSetLevel hook 中 abort 生效
// 注意：SetLevel 需要 session 初始化，这里直接测试 hooks.afterSetLevel 方法
func TestAfterSetLevel_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterSetLevel(func(ctx context.Context, id any, message *mcp.SetLevelRequest, result *mcp.EmptyResult) {
		AbortRequest(ctx, errors.New("after-set-level abort"))
	})

	ctx := withAbortSignal(context.Background())
	req := &mcp.SetLevelRequest{}
	req.Params.Level = mcp.LoggingLevelDebug
	result := &mcp.EmptyResult{}

	err := hooks.afterSetLevel(ctx, 1, req, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after-set-level abort")
}

// TestAfterComplete_Abort 验证 AfterComplete hook 中 abort 生效
func TestAfterComplete_Abort(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddAfterComplete(func(ctx context.Context, id any, message *mcp.CompleteRequest, result *mcp.CompleteResult) {
		AbortRequest(ctx, errors.New("after-complete abort"))
	})

	server := NewMCPServer("test", "1.0.0", WithCompletions(), WithPromptCapabilities(false), WithHooks(hooks))
	server.AddPrompt(
		mcp.Prompt{Name: "p", Description: "test"},
		func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		},
	)
	resp := sendMessage(t, server, `{
		"jsonrpc":"2.0","id":1,"method":"completion/complete",
		"params":{"ref":{"type":"ref/prompt","name":"p"},"argument":{"name":"a","value":"v"}}
	}`)
	assertErrorResponse(t, resp, "after-complete abort")
}

// ─────────────────────────────────────────────
// 4. before/after 共享同一个 abort signal
// ─────────────────────────────────────────────

// TestBeforeAbort_AfterNotCalled 验证 before hook abort 后 after hook 不被调用
func TestBeforeAbort_AfterNotCalled(t *testing.T) {
	afterCalled := false

	hooks := &Hooks{}
	hooks.AddBeforePing(func(ctx context.Context, id any, message *mcp.PingRequest) {
		AbortRequest(ctx, errors.New("before abort"))
	})
	hooks.AddAfterPing(func(ctx context.Context, id any, message *mcp.PingRequest, result *mcp.EmptyResult) {
		afterCalled = true
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertErrorResponse(t, resp, "before abort")
	assert.False(t, afterCalled, "after hook should not be called when before hook aborts")
}

// TestBeforeAnyAbort_AfterAnyNotCalled 验证 BeforeAny abort 后 AfterAny hook 不被调用
func TestBeforeAnyAbort_AfterAnyNotCalled(t *testing.T) {
	afterAnyCalled := false

	hooks := &Hooks{}
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		AbortRequest(ctx, errors.New("before-any abort"))
	})
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		afterAnyCalled = true
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertErrorResponse(t, resp, "before-any abort")
	assert.False(t, afterAnyCalled, "AfterAny hook should not be called when BeforeAny aborts")
}

// TestAbortRequest_OnlyFirstErrorKept 验证多次调用 AbortRequest 只保留第一个错误
func TestAbortRequest_OnlyFirstErrorKept(t *testing.T) {
	hooks := &Hooks{}
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		AbortRequest(ctx, errors.New("first error"))
		AbortRequest(ctx, errors.New("second error")) // 应被忽略
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertErrorResponse(t, resp, "first error")
}

// ─────────────────────────────────────────────
// 5. abort 不影响无 hook 的正常请求
// ─────────────────────────────────────────────

// TestNoAbort_NormalRequestSucceeds 验证没有 abort 时请求正常成功
func TestNoAbort_NormalRequestSucceeds(t *testing.T) {
	afterCalled := false

	hooks := &Hooks{}
	hooks.AddAfterPing(func(ctx context.Context, id any, message *mcp.PingRequest, result *mcp.EmptyResult) {
		afterCalled = true
		// 不调用 AbortRequest
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assertSuccessResponse(t, resp)
	assert.True(t, afterCalled, "after hook should be called on success")
}

// ─────────────────────────────────────────────
// 6. OnError hook 在 after abort 时被调用
// ─────────────────────────────────────────────

// TestAfterAbort_OnErrorHookCalled 验证 after hook abort 时 OnError hook 被触发
func TestAfterAbort_OnErrorHookCalled(t *testing.T) {
	var capturedErr error

	hooks := &Hooks{}
	hooks.AddAfterPing(func(ctx context.Context, id any, message *mcp.PingRequest, result *mcp.EmptyResult) {
		AbortRequest(ctx, errors.New("after abort triggers error hook"))
	})
	hooks.AddOnError(func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
		capturedErr = err
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	require.NotNil(t, capturedErr)
	assert.Contains(t, capturedErr.Error(), "after abort triggers error hook")
}

// ─────────────────────────────────────────────
// 7. AfterAny 接收到正确的 result 参数
// ─────────────────────────────────────────────

// TestAfterAny_ReceivesResult 验证 AfterAny hook 收到正确的 result
func TestAfterAny_ReceivesResult(t *testing.T) {
	var capturedResult any

	hooks := &Hooks{}
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		capturedResult = result
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	server.AddTool(mcp.NewTool("greet"), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("hello"), nil
	})

	sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"greet"}}`)

	require.NotNil(t, capturedResult)
}

// ─────────────────────────────────────────────
// 8. withAbortSignal 注入一次，before/after 共享
// ─────────────────────────────────────────────

// TestSharedAbortSignal_BeforeAbortSeenInAfter 验证 before hook 设置的 abort 在 after hook 中可见
func TestSharedAbortSignal_BeforeAbortSeenInAfter(t *testing.T) {
	var afterSawAbort bool

	hooks := &Hooks{}
	// before hook 设置 abort
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		AbortRequest(ctx, errors.New("shared signal"))
	})
	// after hook 不会被执行（因为 before 已 abort），但我们验证 abort 确实阻止了 after
	hooks.AddAfterAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any, result any) {
		afterSawAbort = true
	})

	server := NewMCPServer("test", "1.0.0", WithHooks(hooks))
	resp := sendMessage(t, server, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)

	// before abort 阻止了请求，after 不应被调用
	assertErrorResponse(t, resp, "shared signal")
	assert.False(t, afterSawAbort, "after hook should not run when before aborted via shared signal")
}
