package tui

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// QueueInfo es una cola SQS desplegada contra floci, con sus contadores
// aproximados (los mismos que expone GetQueueAttributes).
type QueueInfo struct {
	Name     string
	URL      string
	Visible  int
	InFlight int
}

// TopicInfo es un tópico SNS desplegado contra floci. SNS no retiene
// histórico de publicaciones (no hay nada como "ver los últimos mensajes"),
// así que lo único inspeccionable de forma estructural son sus suscriptores
// — cada uno como "protocolo: destino", ej. "sqs: mi-cola".
type TopicInfo struct {
	Name          string
	ARN           string
	Subscriptions []string
}

type sqsMessage struct {
	MessageId string `json:"MessageId"`
	Body      string `json:"Body"`
}

var httpClient = http.Client{Timeout: 3 * time.Second}

// sqsCall pega el protocolo JSON de SQS (POST a la raíz con X-Amz-Target),
// que floci acepta sin autenticación — igual que el invoke de Lambda.
func sqsCall(endpoint, target string, body map[string]any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(endpoint, "/")+"/", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("X-Amz-Target", target)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llamando a %s: %w", target, err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: floci respondió %s: %s", target, resp.Status, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// listQueues pregunta a floci qué colas SQS hay y, para cada una, sus
// contadores aproximados de mensajes visibles/en vuelo.
func listQueues(endpoint string) ([]QueueInfo, error) {
	out, err := sqsCall(endpoint, "AmazonSQS.ListQueues", map[string]any{})
	if err != nil {
		return nil, err
	}
	var listResp struct {
		QueueUrls []string `json:"QueueUrls"`
	}
	if err := json.Unmarshal(out, &listResp); err != nil {
		return nil, fmt.Errorf("leyendo ListQueues: %w", err)
	}

	infos := make([]QueueInfo, 0, len(listResp.QueueUrls))
	for _, qurl := range listResp.QueueUrls {
		attrs, err := sqsGetQueueAttributes(endpoint, qurl, []string{"ApproximateNumberOfMessages", "ApproximateNumberOfMessagesNotVisible"})
		if err != nil {
			continue
		}
		infos = append(infos, QueueInfo{
			Name:     lastPathSegment(qurl),
			URL:      qurl,
			Visible:  atoiOr0(attrs["ApproximateNumberOfMessages"]),
			InFlight: atoiOr0(attrs["ApproximateNumberOfMessagesNotVisible"]),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

func sqsGetQueueAttributes(endpoint, queueURL string, names []string) (map[string]string, error) {
	out, err := sqsCall(endpoint, "AmazonSQS.GetQueueAttributes", map[string]any{
		"QueueUrl":       queueURL,
		"AttributeNames": names,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Attributes map[string]string `json:"Attributes"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("leyendo GetQueueAttributes: %w", err)
	}
	return parsed.Attributes, nil
}

// peekQueueMessages muestra hasta max mensajes de la cola SIN sacarlos de
// circulación: VisibilityTimeout=0 hace que floci los deje visibles de
// nuevo de inmediato para cualquier otro consumidor (probado: pedir dos
// veces seguidas devuelve los mismos mensajes, sin duplicarlos ni
// perderlos). Ojo: sí cuenta como una entrega más a efectos de
// ReceiveCount — si tenés una redrive policy con maxReceiveCount muy chico,
// mirar la cola seguido en el dashboard suma a ese contador.
func peekQueueMessages(endpoint, queueURL string, max int) ([]sqsMessage, error) {
	out, err := sqsCall(endpoint, "AmazonSQS.ReceiveMessage", map[string]any{
		"QueueUrl":            queueURL,
		"MaxNumberOfMessages": max,
		"VisibilityTimeout":   0,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Messages []sqsMessage `json:"Messages"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("leyendo ReceiveMessage: %w", err)
	}
	return parsed.Messages, nil
}

// formatPeekedMessages arma las líneas a mostrar en el panel de logs para
// una cola seleccionada.
func formatPeekedMessages(msgs []sqsMessage) []string {
	if len(msgs) == 0 {
		return []string{"(cola vacía)"}
	}
	lines := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		id := msg.MessageId
		if len(id) > 8 {
			id = id[:8]
		}
		lines = append(lines, id+" │ "+msg.Body)
	}
	return lines
}

// snsQueryCall pega el protocolo "Query" clásico de SNS (POST
// form-urlencoded a la raíz con Action=...), que floci también acepta sin
// autenticación. La respuesta es XML.
func snsQueryCall(endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(endpoint, "/")+"/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llamando a SNS %s: %w", form.Get("Action"), err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SNS %s: floci respondió %s: %s", form.Get("Action"), resp.Status, strings.TrimSpace(string(out)))
	}
	return out, nil
}

type snsListTopicsXML struct {
	Result struct {
		Topics struct {
			Member []struct {
				TopicArn string `xml:"TopicArn"`
			} `xml:"member"`
		} `xml:"Topics"`
	} `xml:"ListTopicsResult"`
}

type snsListSubscriptionsXML struct {
	Result struct {
		Subscriptions struct {
			Member []struct {
				Protocol string `xml:"Protocol"`
				Endpoint string `xml:"Endpoint"`
			} `xml:"member"`
		} `xml:"Subscriptions"`
	} `xml:"ListSubscriptionsByTopicResult"`
}

// listTopics pregunta a floci qué tópicos SNS hay y, para cada uno, sus
// suscriptores actuales (a qué cola/función/endpoint reenvía lo que se
// publique ahí).
func listTopics(endpoint string) ([]TopicInfo, error) {
	out, err := snsQueryCall(endpoint, url.Values{"Action": {"ListTopics"}, "Version": {"2010-03-31"}})
	if err != nil {
		return nil, err
	}
	var parsed snsListTopicsXML
	if err := xml.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("leyendo ListTopics: %w", err)
	}

	infos := make([]TopicInfo, 0, len(parsed.Result.Topics.Member))
	for _, t := range parsed.Result.Topics.Member {
		var subs []string
		subsOut, err := snsQueryCall(endpoint, url.Values{
			"Action":   {"ListSubscriptionsByTopic"},
			"Version":  {"2010-03-31"},
			"TopicArn": {t.TopicArn},
		})
		if err == nil {
			var subsParsed snsListSubscriptionsXML
			if xml.Unmarshal(subsOut, &subsParsed) == nil {
				for _, s := range subsParsed.Result.Subscriptions.Member {
					subs = append(subs, s.Protocol+": "+arnName(s.Endpoint))
				}
			}
		}
		infos = append(infos, TopicInfo{Name: arnName(t.TopicArn), ARN: t.TopicArn, Subscriptions: subs})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// arnName devuelve el último segmento de un ARN (o de una URL de cola SQS,
// que no es un ARN pero comparte la misma idea de "lo último después del
// separador"), que en la práctica es el nombre legible del recurso.
func arnName(arn string) string {
	if idx := strings.LastIndex(arn, ":"); idx != -1 {
		return arn[idx+1:]
	}
	return arn
}

func lastPathSegment(u string) string {
	if idx := strings.LastIndex(u, "/"); idx != -1 {
		return u[idx+1:]
	}
	return u
}

func atoiOr0(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
