package auth

import (
	"testing"
	"time"
)

func TestAuthToken(t *testing.T) {
	a := New("secret", []byte("test-secret"), false)
	if !a.CheckPassword("secret") {
		t.Fatal("正确密码应通过")
	}
	if a.CheckPassword("wrong") {
		t.Fatal("错误密码不应通过")
	}
	tok := a.token(time.Now().Add(time.Hour).Unix())
	if !a.validToken(tok) {
		t.Fatal("合法 token 应通过")
	}
	if a.validToken(tok + "x") {
		t.Fatal("被篡改的 token 不应通过")
	}
	expired := a.token(time.Now().Add(-time.Minute).Unix())
	if a.validToken(expired) {
		t.Fatal("过期 token 不应通过")
	}
	other := New("secret", []byte("other"), false)
	if other.validToken(tok) {
		t.Fatal("不同密钥签发的 token 不应通过")
	}
}
