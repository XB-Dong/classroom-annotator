package config

import (
	"testing"
)

func TestAuthConfigParsesOnlyExplicitTrustedOriginsAndSecureCookies(t *testing.T) {
	values := map[string]string{"APP_DATABASE_URL": "postgres://localhost/test", "APP_DATA_DIR": t.TempDir(), "APP_AUTH_INVITE_CODE": "invite", "APP_AUTH_TRUSTED_ORIGINS": "https://app.test"}
	c, e := Load(func(k string) string { return values[k] })
	if e != nil {
		t.Fatal(e)
	}
	if !c.AuthCookieSecure || c.AuthInviteCode != "invite" || len(c.AuthTrustedOrigins) != 1 {
		t.Fatal("auth defaults missing")
	}
	for _, bad := range []string{"*", "https://app.test/path", "https://u:p@app.test", "https://app.test?x=1"} {
		values["APP_AUTH_TRUSTED_ORIGINS"] = bad
		if _, e = Load(func(k string) string { return values[k] }); e == nil {
			t.Fatalf("invalid origin accepted: %s", bad)
		}
	}
}
