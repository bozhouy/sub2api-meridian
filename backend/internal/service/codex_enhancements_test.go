package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDiagnosticAccountPinRetainsExclusions(t *testing.T) {
 resetOpenAIAdvancedSchedulerSettingCacheForTest()
 groupID:=int64(90901)
 accounts:=[]Account{
  {ID:90001,Platform:PlatformOpenAI,Type:AccountTypeAPIKey,Status:StatusActive,Schedulable:true,Concurrency:1,Priority:10},
  {ID:90002,Platform:PlatformOpenAI,Type:AccountTypeAPIKey,Status:StatusActive,Schedulable:true,Concurrency:1,Priority:0},
 }
 for _,test:=range []struct{name string;target int64;excluded map[int64]struct{};allowed bool}{
  {name:"pins eligible target",target:90001,allowed:true},
  {name:"cannot add target outside eligible pool",target:99999},
  {name:"excluded target cannot fail over to another account",target:90001,excluded:map[int64]struct{}{90001:{}}},
 } { t.Run(test.name,func(t *testing.T){
  svc:=&OpenAIGatewayService{accountRepo:schedulerTestOpenAIAccountRepo{accounts:accounts},cache:&schedulerTestGatewayCache{},cfg:&config.Config{},concurrencyService:NewConcurrencyService(schedulerTestConcurrencyCache{})}
  selection,_,err:=svc.SelectAccountWithScheduler(WithCodexTicketDiagnostic(context.Background(),test.target),&groupID,"","","gpt-6.1-sol",test.excluded,OpenAIUpstreamTransportAny,false)
  if !test.allowed {require.Error(t,err);require.Nil(t,selection);return}
  require.NoError(t,err);require.Equal(t,test.target,selection.Account.ID)
  if selection.ReleaseFunc!=nil {selection.ReleaseFunc()}
 }) }
}

func TestRequestLanguagePresenceAndOptOut(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI}
	header := http.Header{}
	normalizeOpenAIRequestLanguage(account, header)
	require.Empty(t, header)
	header.Set("Accept-Language", "zh-CN")
	header.Set("Authorization", "Bearer example")
	normalizeOpenAIRequestLanguage(account, header)
	require.Equal(t, "en-US,en;q=0.9", header.Get("Accept-Language"))
	require.Equal(t, "Bearer example", header.Get("Authorization"))
	account.Extra = map[string]any{"openai_request_language_enabled": false}
	header.Set("Accept-Language", "fr-FR")
	normalizeOpenAIRequestLanguage(account, header)
	require.Equal(t, "fr-FR", header.Get("Accept-Language"))
	normalizeOpenAIRequestLanguage(&Account{Platform: PlatformAnthropic}, header)
	require.Equal(t, "fr-FR", header.Get("Accept-Language"))
}

func TestRequestTimezoneCanBeDisabled(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_request_timezone_mode": "off"}}
	body := []byte(`{"input":[{"role":"user","content":"<environment_context><timezone>America/New_York</timezone></environment_context>"}]}`)
	require.Equal(t, body, normalizeOpenAIRequestLocale(context.Background(), account, body, "http"))
	require.NoError(t, ValidateOpenAIRequestTimezoneExtra(PlatformOpenAI, account.Extra))
}

func TestStandaloneDiagnosticRequestRendersChallenge(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{"openai_request_timezone": "America/New_York"}}
	challenge, err := NewModelTraceChallenge()
	require.NoError(t, err)
	body, err := BuildStandaloneDiagnosticRequest(account, "gpt-6.1-sol", challenge)
	require.NoError(t, err)
	require.True(t, gjson.ValidBytes(body))
	require.Equal(t, "gpt-6.1-sol", gjson.GetBytes(body, "model").String())
	require.True(t, gjson.GetBytes(body, "stream").Bool())
	require.False(t, gjson.GetBytes(body, "store").Bool())
	require.NotContains(t, string(body), "{{")
	require.Contains(t, string(body), "America/New_York")
	input := gjson.GetBytes(body, "input").Array()
	require.Equal(t, challenge.Prompt, input[len(input)-1].Get("content.0.text").String())
	require.False(t, strings.Contains(string(body), "access_token"))
}
