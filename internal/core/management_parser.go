package core

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// passwordPrompt describe un pedido de credenciales del management.
// Viene de líneas tipo:
//
//	>PASSWORD:Need 'Auth' username/password
//	>PASSWORD:Need 'Auth' username/password SC:1,Please enter your OTP
//
// Y, para desafíos dinámicos post-fallo:
//
//	>PASSWORD:Verification Failed: 'Auth' ['CRV1:R,E:<state>:<user_b64>:<challenge>']
type passwordPrompt struct {
	realm           string // Típicamente "Auth".
	needsCredential bool   // true si pide user+password nuevo.
	staticChallenge string // Texto del challenge estático (vacío si no hay).

	// Dynamic challenge (CRV1).
	crv1        bool
	crv1State   string // Identificador opaco que debe repetirse.
	crv1UserB64 string // Usuario codificado en base64 según envió el server.
	crv1Text    string // Texto del challenge para mostrarle al usuario.
}

// parsePasswordPrompt interpreta una línea >PASSWORD:... del management.
// Devuelve nil si la línea no corresponde a un pedido de credencial.
func parsePasswordPrompt(line string) *passwordPrompt {
	payload := strings.TrimPrefix(line, ">PASSWORD:")
	if payload == line {
		return nil
	}

	// Caso 1: Need 'Realm' username/password [SC:1,challenge]
	if strings.HasPrefix(payload, "Need '") {
		p := &passwordPrompt{needsCredential: true}
		rest := strings.TrimPrefix(payload, "Need '")
		end := strings.Index(rest, "'")
		if end < 0 {
			return nil
		}
		p.realm = rest[:end]
		rest = rest[end+1:] // algo como " username/password SC:1,text" o " username/password"
		if idx := strings.Index(rest, "SC:1,"); idx >= 0 {
			p.staticChallenge = rest[idx+len("SC:1,"):]
		}
		return p
	}

	// Caso 2: Verification Failed: 'Realm' ['CRV1:...']
	if strings.HasPrefix(payload, "Verification Failed: '") {
		p := &passwordPrompt{}
		rest := strings.TrimPrefix(payload, "Verification Failed: '")
		end := strings.Index(rest, "'")
		if end < 0 {
			return nil
		}
		p.realm = rest[:end]
		// Buscar el payload del CRV1 entre comillas simples.
		if i := strings.Index(rest, "'CRV1:"); i >= 0 {
			tail := rest[i+1:]
			if j := strings.LastIndex(tail, "'"); j > 0 {
				crv := tail[:j]
				if parseCRV1(crv, p) {
					return p
				}
			}
		}
		// Verificación fallida sin CRV1: credenciales inválidas "puras".
		return p
	}

	return nil
}

// parseCRV1 descompone "CRV1:R,E:<state>:<user_b64>:<challenge_text>".
func parseCRV1(crv string, out *passwordPrompt) bool {
	if !strings.HasPrefix(crv, "CRV1:") {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(crv, "CRV1:"), ":", 4)
	if len(parts) < 4 {
		return false
	}
	// parts[0] = flags (p.ej. "R,E"); parts[1] = state; parts[2] = user_b64; parts[3] = challenge_text.
	out.crv1 = true
	out.crv1State = parts[1]
	out.crv1UserB64 = parts[2]
	out.crv1Text = parts[3]
	return true
}

// bytecountFromLine parsea ">BYTECOUNT:<in>,<out>".
func bytecountFromLine(line string) (in, out uint64, ok bool) {
	payload := strings.TrimPrefix(line, ">BYTECOUNT:")
	if payload == line {
		return 0, 0, false
	}
	parts := strings.SplitN(payload, ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	a, err1 := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	b, err2 := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return a, b, true
}

// stateFromLine parsea ">STATE:<ts>,<name>,<desc>,<local_tun>,<remote_server>,[remote_port],[local_port],[local_ip6]".
// Devuelve el nombre del estado y los IPs cuando están disponibles.
func stateFromLine(line string) (name, localTun, remoteServer string, ok bool) {
	payload := strings.TrimPrefix(line, ">STATE:")
	if payload == line {
		return "", "", "", false
	}
	fields := strings.Split(payload, ",")
	if len(fields) < 2 {
		return "", "", "", false
	}
	name = fields[1]
	if len(fields) >= 4 {
		localTun = fields[3]
	}
	if len(fields) >= 5 {
		remoteServer = fields[4]
	}
	return name, localTun, remoteServer, true
}

// encodeCommandArg cita un argumento para los comandos del management interface,
// que usan comillas dobles y backslash escapes. openvpn no permite \n ni \r
// dentro del argumento.
func encodeCommandArg(s string) string {
	// Reemplazar backslash primero, luego la comilla.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// buildStaticChallengeResponse construye el valor SCRV1 con password y OTP
// codificados en base64: "SCRV1:base64(pass):base64(otp)".
func buildStaticChallengeResponse(password, otp string) string {
	return fmt.Sprintf("SCRV1:%s:%s",
		base64.StdEncoding.EncodeToString([]byte(password)),
		base64.StdEncoding.EncodeToString([]byte(otp)))
}

// buildDynamicChallengeResponse construye el valor CRV1 para responder al
// challenge dinámico: "CRV1::<state>::<otp>".
func buildDynamicChallengeResponse(state, otp string) string {
	return fmt.Sprintf("CRV1::%s::%s", state, otp)
}
