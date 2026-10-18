package approvalforgery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/aluiziodeveloper/guarita/examples/naiveapproval"
	"github.com/aluiziodeveloper/guarita/internal/adapters/admin"
	"github.com/aluiziodeveloper/guarita/internal/adapters/approval"
	"github.com/aluiziodeveloper/guarita/internal/adapters/mcphttp"
	"github.com/aluiziodeveloper/guarita/internal/domain"
)

const anaSecret = "segredo-da-ana-0123456789"

// operatorInterface monta a interface do operador com as camadas da
// linha de comando: guarda de Host, proteção de origem e credencial.
func operatorInterface(t *testing.T, queue *approval.Queue) string {
	t.Helper()
	operators, _ := admin.ParseOperators("ana " + anaSecret)
	server := httptest.NewUnstartedServer(nil)
	hosts, _ := mcphttp.LoopbackHosts(server.Listener.Addr().String())
	guard, _ := mcphttp.NewGuard(hosts, nil)
	server.Config.Handler = guard.Wrap(http.NewCrossOriginProtection().
		Handler(admin.NewHandler(queue, operators)))
	server.Start()
	t.Cleanup(server.Close)
	return server.URL
}

// pending põe um delete_database do agente na fila e espera chegar.
func pending(t *testing.T, queue *approval.Queue) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_, _ = queue.Await(ctx, domain.PolicyInput{
			Agent:  "manutencao",
			Server: "db", Call: domain.Call{ToolName: "delete_database"},
		})
	}()
	for range 100 {
		if p := queue.Pending(); len(p) == 1 {
			return p[0].ID
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("o pedido não chegou à fila")
	return ""
}

// post tenta aprovar com os cabeçalhos dados e devolve o estado.
func post(t *testing.T, url string, header map[string]string) int {
	t.Helper()
	r, _ := http.NewRequest("POST", url, nil)
	for name, value := range header {
		if name == "Host" {
			r.Host = value
			continue
		}
		r.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}

// #region attacks-forgery-test
// Na fila ingênua, o agente aprova a si mesmo. Na da Guarita, cada
// tentativa de forja é recusada por uma camada diferente, o pedido
// segue pendente, e só a ana aprova, uma vez.
func TestForgery(t *testing.T) {
	naive := &naiveapproval.Queue{}
	id := naive.Request()
	naive.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET",
		"/approve?id="+strconv.Itoa(id), nil))
	t.Logf("ingênua: o agente aprovou o próprio pedido? %v",
		naive.Approved(id))

	queue := approval.New(time.Minute)
	base := operatorInterface(t, queue)
	approve := base + "/approvals/" + pending(t, queue) + "/approve"
	ana := "Bearer " + anaSecret
	for _, a := range attempts(ana) {
		code := post(t, approve, a.header)
		t.Logf("%-22s -> %d", a.name, code)
		forged := a.want == 401 || a.want == 403
		if code != a.want || (forged && len(queue.Pending()) != 1) {
			t.Errorf("%s: %d, want %d", a.name, code, a.want)
		}
	}
}

// #endregion
