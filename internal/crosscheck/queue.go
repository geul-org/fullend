package crosscheck

import (
	"fmt"
	"strings"

	ssacparser "github.com/geul-org/ssac/parser"
)

// CheckQueue validates publish ↔ subscribe cross-references.
func CheckQueue(funcs []ssacparser.ServiceFunc, queueBackend string) []CrossError {
	var errs []CrossError

	// Collect publish topics: topic → set of payload field names.
	publishTopics := map[string]map[string]bool{}
	// Collect subscribe topics: topic → ServiceFunc.
	subscribeTopics := map[string]ssacparser.ServiceFunc{}

	for _, fn := range funcs {
		if fn.Subscribe != nil {
			subscribeTopics[fn.Subscribe.Topic] = fn
		}
		for _, seq := range fn.Sequences {
			if seq.Type == "publish" {
				fields := map[string]bool{}
				for k := range seq.Inputs {
					fields[k] = true
				}
				publishTopics[seq.Topic] = fields
			}
		}
	}

	hasQueue := len(publishTopics) > 0 || len(subscribeTopics) > 0

	// Rule: queue not configured but @publish/@subscribe used.
	if hasQueue && queueBackend == "" {
		errs = append(errs, CrossError{
			Rule:       "Queue config",
			Context:    "fullend.yaml",
			Message:    "fullend.yaml에 queue 설정이 없지만 @publish/@subscribe가 사용되었습니다",
			Level:      "ERROR",
			Suggestion: "fullend.yaml에 queue.backend을 설정하세요 (예: postgres, memory)",
		})
	}

	// Rule: publish topic has no matching subscribe.
	for topic := range publishTopics {
		if _, ok := subscribeTopics[topic]; !ok {
			errs = append(errs, CrossError{
				Rule:       "Queue publish → subscribe",
				Context:    topic,
				Message:    fmt.Sprintf("@publish 토픽 %q에 대한 @subscribe 함수가 없습니다", topic),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("토픽 %q를 구독하는 @subscribe 함수를 추가하세요", topic),
			})
		}
	}

	// Rule: subscribe topic has no matching publish.
	for topic := range subscribeTopics {
		if _, ok := publishTopics[topic]; !ok {
			errs = append(errs, CrossError{
				Rule:       "Queue subscribe → publish",
				Context:    topic,
				Message:    fmt.Sprintf("@subscribe 토픽 %q에 대한 @publish가 없습니다", topic),
				Level:      "WARNING",
				Suggestion: fmt.Sprintf("토픽 %q를 발행하는 @publish 시퀀스를 추가하세요", topic),
			})
		}
	}

	// Rule: subscribe message struct fields ↔ publish payload fields.
	for topic, fn := range subscribeTopics {
		pubFields, ok := publishTopics[topic]
		if !ok {
			continue // already warned about missing publish
		}

		// Find the message struct matching the param type.
		typeName := ""
		if fn.Param != nil {
			typeName = fn.Param.TypeName
		}
		if typeName == "" {
			continue
		}

		var structFields []string
		for _, st := range fn.Structs {
			if st.Name == typeName {
				for _, f := range st.Fields {
					structFields = append(structFields, f.Name)
				}
				break
			}
		}

		// Check each subscribe struct field exists in publish payload.
		for _, fieldName := range structFields {
			if !pubFields[fieldName] {
				errs = append(errs, CrossError{
					Rule:    "Queue field mismatch",
					Context: fmt.Sprintf("%s.%s", topic, fieldName),
					Message: fmt.Sprintf(
						"@subscribe 메시지 필드 %q가 @publish 토픽 %q의 payload에 없습니다 (payload: %s)",
						fieldName, topic, joinKeys(pubFields),
					),
					Level:      "WARNING",
					Suggestion: fmt.Sprintf("@publish payload에 %q 필드를 추가하거나 @subscribe 메시지 struct에서 제거하세요", fieldName),
				})
			}
		}
	}

	return errs
}

// joinKeys returns sorted comma-joined keys of a map.
func joinKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}
