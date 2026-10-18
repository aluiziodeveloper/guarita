package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aluiziodeveloper/guarita/internal/adapters/approval"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

const anaSecret = "segredo-da-ana-0123456789"

var operators = Operators{}

func init() {
	ops, err := ParseOperators("# operadores do laboratório\n" +
		"ana " + anaSecret + "\n" +
		"manutencao segredo-do-agente-0123456789\n")
	if err != nil {
		panic(err)
	}
	operators = ops
}

// waiting põe um pedido do agente na fila e espera ele chegar lá.
func waiting(
	t *testing.T, queue *approval.Queue, arguments string,
) <-chan domain.ApprovalState {
	t.Helper()
	result := make(chan domain.ApprovalState, 1)
	go func() {
		state, _ := queue.Await(t.Context(), domain.PolicyInput{
			Agent: "manutencao", Server: "db",
			Call: domain.Call{
				ToolName:  "delete_database",
				Arguments: json.RawMessage(arguments),
			},
		})
		result <- state
	}()
	synctest.Wait()
	return result
}

// do chama o handler sem rede, com a credencial dada.
func do(h http.Handler, method, path, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// #region admin-flow-test
// O operador lista, aprova e tenta aprovar de novo; o agente, que
// também tem nome de operador neste laboratório, não decide o próprio
// pedido; um identificador inventado não existe.
func TestHandler_fluxoDoOperador(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := approval.New(time.Hour)
		h := NewHandler(queue, operators)
		result := waiting(t, queue, `{"confirm":true}`)
		ana := "Bearer " + anaSecret
		list := do(h, "GET", "/approvals", ana)
		var items []Item
		_ = json.Unmarshal(list.Body.Bytes(), &items)
		path := "/approvals/" + items[0].ID
		self := do(h, "POST", path+"/approve",
			"Bearer segredo-do-agente-0123456789")
		first := do(h, "POST", path+"/approve", ana)
		again := do(h, "POST", path+"/deny", ana)
		guess := do(h, "POST", "/approvals/1/approve", ana)
		t.Logf("lista %d (%d pedido); agente %d; ana %d -> %s; "+
			"de novo %d; id inventado %d", list.Code, len(items),
			self.Code, first.Code, <-result, again.Code, guess.Code)
		if self.Code != 403 || first.Code != 204 || again.Code != 409 ||
			guess.Code != 404 {
			t.Fatal("fluxo fora do contrato")
		}
	})
}

// #endregion

// #region admin-auth-test
// Sem credencial, com o token de um agente, ou com o segredo certo e o
// nome errado no Basic, a interface responde 401.
func TestHandler_exigeOperador(t *testing.T) {
	h := NewHandler(approval.New(time.Hour), operators)
	basic := func(user, pass string) string {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetBasicAuth(user, pass)
		return r.Header.Get("Authorization")
	}
	for name, auth := range map[string]string{
		"sem credencial":    "",
		"token de agente":   "Bearer eyJhbGciOiJFZERTQSJ9.e30.x",
		"basic, outro nome": basic("bruno", anaSecret),
		"basic, ana":        basic("ana", anaSecret),
	} {
		code := do(h, "GET", "/approvals", auth).Code
		t.Logf("%-17s -> %d", name, code)
		if (code == 200) != (name == "basic, ana") {
			t.Errorf("%s: %d", name, code)
		}
	}
}

// #endregion

// #region admin-page-test
// Os argumentos vêm do agente. Na página, a marcação deles aparece
// escapada, e não como HTML.
func TestHandler_paginaEscapaArgumentos(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		queue := approval.New(time.Hour)
		waiting(t, queue, `{"note":"<script>[LAB]</script>"}`)
		body := do(NewHandler(queue, operators), "GET", "/",
			"Bearer "+anaSecret).Body.String()
		line := body[strings.Index(body, "<pre>"):]
		line = line[:strings.Index(line, "</pre>")+6]
		t.Logf("na página: %s", line)
		if strings.Contains(body, "<script>") {
			t.Fatal("a marcação do agente chegou crua à página")
		}
	})
}

// #endregion
