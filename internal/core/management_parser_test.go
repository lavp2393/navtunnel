package core

import "testing"

func TestParsePasswordPrompt_Need_NoChallenge(t *testing.T) {
	p := parsePasswordPrompt(">PASSWORD:Need 'Auth' username/password")
	if p == nil {
		t.Fatal("parsePasswordPrompt devolvió nil")
	}
	if p.realm != "Auth" {
		t.Errorf("realm = %q, quería Auth", p.realm)
	}
	if !p.needsCredential {
		t.Error("needsCredential debería ser true")
	}
	if p.staticChallenge != "" {
		t.Errorf("no debería haber static challenge, got %q", p.staticChallenge)
	}
	if p.crv1 {
		t.Error("no debería ser CRV1")
	}
}

func TestParsePasswordPrompt_Need_WithStaticChallenge(t *testing.T) {
	p := parsePasswordPrompt(">PASSWORD:Need 'Auth' username/password SC:1,Please enter your 2FA code")
	if p == nil {
		t.Fatal("nil")
	}
	if !p.needsCredential {
		t.Error("needsCredential")
	}
	if p.staticChallenge != "Please enter your 2FA code" {
		t.Errorf("staticChallenge = %q", p.staticChallenge)
	}
}

func TestParsePasswordPrompt_DynamicChallenge(t *testing.T) {
	line := ">PASSWORD:Verification Failed: 'Auth' ['CRV1:R,E:session-abc-123:dXNlcg==:Enter your OTP']"
	p := parsePasswordPrompt(line)
	if p == nil {
		t.Fatal("nil")
	}
	if !p.crv1 {
		t.Fatal("debería ser CRV1")
	}
	if p.crv1State != "session-abc-123" {
		t.Errorf("state = %q", p.crv1State)
	}
	if p.crv1UserB64 != "dXNlcg==" {
		t.Errorf("user_b64 = %q", p.crv1UserB64)
	}
	if p.crv1Text != "Enter your OTP" {
		t.Errorf("text = %q", p.crv1Text)
	}
}

func TestParsePasswordPrompt_VerificationFailedNoCRV(t *testing.T) {
	// Tras fallo simple (credenciales malas puras), openvpn puede emitir esto
	// sin CRV1 antes de volver a pedir con Need 'Auth'.
	p := parsePasswordPrompt(">PASSWORD:Verification Failed: 'Auth' []")
	if p == nil {
		t.Fatal("nil")
	}
	if p.needsCredential {
		t.Error("no debería pedir credencial todavía")
	}
	if p.crv1 {
		t.Error("no debería ser CRV1")
	}
	if p.realm != "Auth" {
		t.Errorf("realm = %q", p.realm)
	}
}

func TestParsePasswordPrompt_OtherRealm(t *testing.T) {
	p := parsePasswordPrompt(">PASSWORD:Need 'Private Key' password")
	if p == nil || p.realm != "Private Key" {
		t.Errorf("realm = %v", p)
	}
}

func TestParsePasswordPrompt_NonPasswordLine(t *testing.T) {
	if parsePasswordPrompt(">STATE:1,CONNECTED,SUCCESS,10.8.0.2,1.2.3.4") != nil {
		t.Error("línea de estado no es un prompt")
	}
}

func TestBytecountFromLine(t *testing.T) {
	in, out, ok := bytecountFromLine(">BYTECOUNT:1024,2048")
	if !ok || in != 1024 || out != 2048 {
		t.Errorf("got in=%d out=%d ok=%v", in, out, ok)
	}
	if _, _, ok := bytecountFromLine(">STATE:..."); ok {
		t.Error("no debería parsear >STATE")
	}
	if _, _, ok := bytecountFromLine(">BYTECOUNT:notnumbers,42"); ok {
		t.Error("no debería parsear números inválidos")
	}
}

func TestStateFromLine(t *testing.T) {
	name, tun, remote, ok := stateFromLine(">STATE:1700000000,CONNECTED,SUCCESS,10.8.0.2,vpn.example.com,1194")
	if !ok {
		t.Fatal("ok = false")
	}
	if name != "CONNECTED" {
		t.Errorf("name = %q", name)
	}
	if tun != "10.8.0.2" {
		t.Errorf("tun = %q", tun)
	}
	if remote != "vpn.example.com" {
		t.Errorf("remote = %q", remote)
	}
}

func TestStateFromLine_Partial(t *testing.T) {
	name, tun, remote, ok := stateFromLine(">STATE:1700000000,CONNECTING,,,")
	if !ok || name != "CONNECTING" {
		t.Fatalf("name = %q ok = %v", name, ok)
	}
	if tun != "" || remote != "" {
		t.Errorf("esperaba tun/remote vacíos, got %q/%q", tun, remote)
	}
}

func TestEncodeCommandArg(t *testing.T) {
	cases := []struct{ in, want string }{
		{`hello`, `"hello"`},
		{`has "quote"`, `"has \"quote\""`},
		{`back\slash`, `"back\\slash"`},
		{``, `""`},
	}
	for _, c := range cases {
		if got := encodeCommandArg(c.in); got != c.want {
			t.Errorf("encodeCommandArg(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildStaticChallengeResponse(t *testing.T) {
	// SCRV1:base64(pass):base64(otp)
	got := buildStaticChallengeResponse("pw", "123456")
	want := "SCRV1:cHc=:MTIzNDU2"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBuildDynamicChallengeResponse(t *testing.T) {
	got := buildDynamicChallengeResponse("state-xyz", "987654")
	want := "CRV1::state-xyz::987654"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
