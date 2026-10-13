package store

import (
	"github.com/XB-Dong/classroom-annotator/backend/internal/auth"
	"github.com/XB-Dong/classroom-annotator/backend/internal/platform/testdb"
	"testing"
)

func TestWorkspaceListsExcludeOtherWorkspaceResources(t *testing.T) {
	s, e := Open(t.TempDir(), Options{DatabaseURL: testdb.URL(t)})
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	for _, q := range []string{`INSERT INTO workspaces(id,name) VALUES('other','Other')`, `INSERT INTO videos(id,name,width,height,duration_seconds,status,created_at,storage_key,workspace_id) VALUES('v','Private',2,2,1,'ready','old','videos/v','other')`, `INSERT INTO jobs(id,kind,video_id,batch_id,status,created_at) VALUES('j','extraction','v','b','completed','old')`} {
		if _, e = s.DB.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	v, e := s.VideosInWorkspace(auth.DefaultWorkspaceID)
	if e != nil || len(v) != 0 {
		t.Fatal("foreign video leaked", e)
	}
	j, e := s.JobsInWorkspace(auth.DefaultWorkspaceID)
	if e != nil || len(j) != 0 {
		t.Fatal("foreign job leaked", e)
	}
	v, e = s.VideosInWorkspace("other")
	if e != nil || len(v) != 1 {
		t.Fatal("own video missing", e)
	}
}
