package articleworkflow

import (
	"encoding/json"
	"fmt"
	"testing"

	"wood-passage-creator/internal/module/article"
	"wood-passage-creator/internal/port"
)

func TestProgressEventWireContract(t *testing.T) {
	tests := []struct {
		payload  ProgressPayload
		name     EventName
		dataJSON string
		terminal bool
	}{
		{
			TaskSnapshot{Status: article.StatusPending}, EventTaskSnapshot,
			`{"status":"PENDING","terminal":false,"imageSummary":{"total":0,"settled":0,"succeeded":0,"skipped":0}}`, false,
		},
		{
			TitlesDone{Options: []article.TitleOption{{MainTitle: "Title", SubTitle: "Subtitle"}}},
			EventTitlesDone, `{"options":[{"mainTitle":"Title","subTitle":"Subtitle"}]}`, true,
		},
		{
			OutlineSnapshot{Sections: []OutlinePreviewSection{{Title: new("Draft")}}},
			EventOutlineSnapshot, `{"sections":[{"title":"Draft"}]}`, false,
		},
		{
			OutlineDone{Sections: []article.OutlineSection{{Section: 1, Title: "Intro", Points: []string{"Point"}}}},
			EventOutlineDone, `{"sections":[{"section":1,"title":"Intro","points":["Point"]}]}`, true,
		},
		{
			ContentDelta{Delta: "Text"}, EventContentDelta, `{"delta":"Text"}`, false,
		},
		{
			ContentGenerated{Content: "Full body"}, EventContentGenerated, `{"content":"Full body"}`, false,
		},
		{
			ImagesPlanned{Tasks: []ImageTask{}}, EventImagesPlanned, `{"tasks":[]}`, false,
		},
		{
			ImageFinished{
				Outcome: skippedOutcome(),
				Summary: ImageSummary{Total: 1, Settled: 1, Skipped: 1},
			},
			EventImageFinished,
			`{"outcome":{"task":{"id":"image-1","position":1,"slotId":"cover","kind":"cover","brief":"A woodworking bench","preferredMethod":"PEXELS"},"status":"skipped","warning":{"code":"IMAGE_SKIPPED","imageId":"image-1","position":1,"message":"No usable image after bounded attempts"}},"summary":{"total":1,"settled":1,"succeeded":0,"skipped":1}}`,
			false,
		},
		{
			ImagesDone{Outcomes: []ImageOutcome{}}, EventImagesDone,
			`{"outcomes":[],"summary":{"total":0,"settled":0,"succeeded":0,"skipped":0}}`, false,
		},
		{
			ContentDone{
				Content: "Body", FullContent: "Body", Images: []GeneratedImage{},
				Warnings: []article.GenerationWarning{*skippedOutcome().Warning},
			},
			EventContentDone,
			`{"content":"Body","fullContent":"Body","images":[],"warnings":[{"code":"IMAGE_SKIPPED","imageId":"image-1","position":1,"message":"No usable image after bounded attempts"}]}`,
			true,
		},
		{
			TaskError{Code: "GENERATION_FAILED", Message: "Unable to generate article"},
			EventTaskError, `{"code":"GENERATION_FAILED","message":"Unable to generate article"}`, true,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			event := Event[ProgressPayload]{
				RunID: "run-1", Seq: 1, Phase: article.PhaseContentGenerating, Data: tt.payload,
			}
			if event.Name() != tt.name || event.IsTerminal() != tt.terminal {
				t.Fatalf("event = %s, terminal=%v", event.Name(), event.IsTerminal())
			}
			raw, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf(`{"runId":"run-1","seq":1,"phase":"CONTENT_GENERATING","data":%s}`, tt.dataJSON)
			if string(raw) != want {
				t.Fatalf("event JSON = %s, want %s", raw, want)
			}
		})
	}
}

func TestSnapshotCanTerminateAStageWithoutACompletionEvent(t *testing.T) {
	for _, phase := range []article.ArticlePhase{
		article.PhaseTitleSelecting, article.PhaseOutlineEditing, article.PhaseCompleted,
	} {
		event := Event[TaskSnapshot]{
			RunID: "run-1", Phase: phase,
			Data: TaskSnapshot{Status: article.StatusProcessing, Terminal: true},
		}
		if !event.IsTerminal() {
			t.Fatalf("snapshot for %s must end the subscription", phase)
		}
	}
}

func TestSucceededOutcomeOmitsWarning(t *testing.T) {
	outcome := ImageOutcome{
		Task: coverTask(), Status: ImageSucceeded,
		Asset: &ImageAsset{URL: "https://example.com/bench.jpg", Method: port.MethodPexels},
	}
	raw, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["warning"]; ok {
		t.Fatal("succeeded outcome must omit warning")
	}
	if _, ok := fields["asset"]; !ok {
		t.Fatal("succeeded outcome must include asset")
	}
}
