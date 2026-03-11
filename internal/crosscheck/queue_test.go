package crosscheck

import (
	"testing"

	ssacparser "github.com/geul-org/ssac/parser"
)

func TestCheckQueueNoConfig(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name: "CreateOrder",
			Sequences: []ssacparser.Sequence{
				{
					Type:   "publish",
					Topic:  "order.completed",
					Inputs: map[string]string{"Email": "order.Email", "OrderID": "order.ID"},
				},
			},
		},
		{
			Name:      "OnOrderCompleted",
			Subscribe: &ssacparser.SubscribeInfo{Topic: "order.completed", MessageType: "OnOrderCompletedMessage"},
			Param:     &ssacparser.ParamInfo{TypeName: "OnOrderCompletedMessage", VarName: "message"},
			Structs: []ssacparser.StructInfo{
				{
					Name: "OnOrderCompletedMessage",
					Fields: []ssacparser.StructField{
						{Name: "Email", Type: "string"},
						{Name: "OrderID", Type: "int64"},
					},
				},
			},
		},
	}

	errs := CheckQueue(funcs, "")

	var found bool
	for _, e := range errs {
		if e.Level == "ERROR" && e.Rule == "Queue config" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ERROR for missing queue config, got: %v", errs)
	}
}

func TestCheckQueuePublishNoSubscriber(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name: "CreateOrder",
			Sequences: []ssacparser.Sequence{
				{
					Type:   "publish",
					Topic:  "order.completed",
					Inputs: map[string]string{"Email": "order.Email"},
				},
			},
		},
	}

	errs := CheckQueue(funcs, "postgres")

	var found bool
	for _, e := range errs {
		if e.Level == "WARNING" && e.Rule == "Queue publish → subscribe" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARNING for publish without subscriber, got: %v", errs)
	}
}

func TestCheckQueueSubscribeNoPublisher(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name:      "OnOrderCompleted",
			Subscribe: &ssacparser.SubscribeInfo{Topic: "order.completed", MessageType: "OnOrderCompletedMessage"},
			Param:     &ssacparser.ParamInfo{TypeName: "OnOrderCompletedMessage", VarName: "message"},
			Structs: []ssacparser.StructInfo{
				{
					Name:   "OnOrderCompletedMessage",
					Fields: []ssacparser.StructField{{Name: "Email", Type: "string"}},
				},
			},
		},
	}

	errs := CheckQueue(funcs, "postgres")

	var found bool
	for _, e := range errs {
		if e.Level == "WARNING" && e.Rule == "Queue subscribe → publish" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARNING for subscribe without publisher, got: %v", errs)
	}
}

func TestCheckQueueFieldMismatch(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name: "CreateOrder",
			Sequences: []ssacparser.Sequence{
				{
					Type:   "publish",
					Topic:  "order.completed",
					Inputs: map[string]string{"Email": "order.Email"},
				},
			},
		},
		{
			Name:      "OnOrderCompleted",
			Subscribe: &ssacparser.SubscribeInfo{Topic: "order.completed", MessageType: "OnOrderCompletedMessage"},
			Param:     &ssacparser.ParamInfo{TypeName: "OnOrderCompletedMessage", VarName: "message"},
			Structs: []ssacparser.StructInfo{
				{
					Name: "OnOrderCompletedMessage",
					Fields: []ssacparser.StructField{
						{Name: "Email", Type: "string"},
						{Name: "OrderID", Type: "int64"},
					},
				},
			},
		},
	}

	errs := CheckQueue(funcs, "postgres")

	var found bool
	for _, e := range errs {
		if e.Level == "WARNING" && e.Rule == "Queue field mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WARNING for field mismatch, got: %v", errs)
	}
}

func TestCheckQueueAllMatch(t *testing.T) {
	funcs := []ssacparser.ServiceFunc{
		{
			Name: "CreateOrder",
			Sequences: []ssacparser.Sequence{
				{
					Type:   "publish",
					Topic:  "order.completed",
					Inputs: map[string]string{"Email": "order.Email", "OrderID": "order.ID"},
				},
			},
		},
		{
			Name:      "OnOrderCompleted",
			Subscribe: &ssacparser.SubscribeInfo{Topic: "order.completed", MessageType: "OnOrderCompletedMessage"},
			Param:     &ssacparser.ParamInfo{TypeName: "OnOrderCompletedMessage", VarName: "message"},
			Structs: []ssacparser.StructInfo{
				{
					Name: "OnOrderCompletedMessage",
					Fields: []ssacparser.StructField{
						{Name: "Email", Type: "string"},
						{Name: "OrderID", Type: "int64"},
					},
				},
			},
		},
	}

	errs := CheckQueue(funcs, "postgres")

	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}
