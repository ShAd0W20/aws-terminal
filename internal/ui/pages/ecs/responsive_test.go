package ecs

import "testing"

func TestTaskColumnsPrioritizeTaskDefinitionOnConstrainedWidths(t *testing.T) {
	cols := taskColumnsForWidth(80)
	if len(cols) == 0 || cols[0].Title != "Task definition" {
		t.Fatalf("expected task definition to be first at constrained widths, got %#v", cols)
	}
	for _, col := range cols {
		if col.Title == "Task" || col.Title == "Created" || col.Title == "Started" || col.Title == "IP" {
			t.Fatalf("expected secondary columns hidden at constrained widths, got %#v", cols)
		}
	}
	if cols[0].Width < 30 || cols[0].Width > 38 {
		t.Fatalf("task definition column should be useful but bounded: %#v", cols)
	}
}

func TestTaskDefinitionColumnDoesNotConsumeWideTables(t *testing.T) {
	cols := taskColumnsForWidth(140)
	if len(cols) == 0 || cols[0].Title != "Task definition" {
		t.Fatalf("expected task definition to remain first, got %#v", cols)
	}
	if cols[0].Width > 44 {
		t.Fatalf("task definition column should leave room for task/ip/time context: %#v", cols)
	}
}
