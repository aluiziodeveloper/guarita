package naiveapproval

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// #region examples-naiveapproval-test
// O agente pede, adivinha o próprio número e aprova a si mesmo pelo
// mesmo servidor. Depois, pede cinquenta vezes: a fila aceita todas.
func TestQueue_agenteSeAprova(t *testing.T) {
	queue := &Queue{}
	server := httptest.NewServer(queue)
	t.Cleanup(server.Close)

	id := queue.Request() // delete_database espera aprovação
	response, err := http.Get(server.URL + "/approve?id=" +
		strconv.Itoa(id))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	t.Logf("pedido %d aprovado pelo próprio agente? %v",
		id, queue.Approved(id))

	for range 50 {
		queue.Request()
	}
	t.Logf("pendentes depois da insistência: %d", queue.Pending())
	if !queue.Approved(id) || queue.Pending() != 50 {
		t.Fatal("a fila ingênua deveria aceitar as duas coisas")
	}
}

// #endregion
