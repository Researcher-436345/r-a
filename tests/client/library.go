package client

// Folder is one row of GET /library/folders.
type Folder struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ParentID     *string `json:"parent_id"`
	SystemKey    *string `json:"system_key"`
	ArticleCount int     `json:"article_count"`
}

// Folders lists the user's folders (system ones first).
func (s *Session) Folders() ([]Folder, Response) {
	s.t.Helper()
	resp := s.Get("/library/folders")
	var out struct {
		Items []Folder `json:"items"`
	}
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out.Items, resp
}

// SystemFolder returns the folder with the given system key ("want_to_read",
// "reading", "other"); fails the test when it is missing.
func (s *Session) SystemFolder(key string) Folder {
	s.t.Helper()
	folders, resp := s.Folders()
	if resp.Status != 200 {
		s.t.Fatalf("GET /library/folders: %s", resp)
	}
	for _, f := range folders {
		if f.SystemKey != nil && *f.SystemKey == key {
			return f
		}
	}
	s.t.Fatalf("no system folder %q among %d folders", key, len(folders))
	return Folder{}
}

// CreateFolder is the "new folder" action; parentID may be empty.
func (s *Session) CreateFolder(name, parentID string) (Folder, Response) {
	s.t.Helper()
	body := map[string]any{"name": name}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	resp := s.Post("/library/folders", body)
	var f Folder
	if resp.Status == 201 {
		resp.JSON(s.t, &f)
	}
	return f, resp
}

// DeleteFolder removes a folder.
func (s *Session) DeleteFolder(id string) Response {
	s.t.Helper()
	return s.Delete("/library/folders/" + id)
}

// AddToLibrary is the "save" button; folderID may be empty.
func (s *Session) AddToLibrary(paperID, folderID string) (LibraryItem, Response) {
	s.t.Helper()
	body := map[string]any{}
	if folderID != "" {
		body["folder_id"] = folderID
	}
	resp := s.Post("/library/"+paperID, body)
	var it LibraryItem
	if resp.Status == 201 {
		resp.JSON(s.t, &it)
	}
	return it, resp
}

// LibraryItemOf reads one library row.
func (s *Session) LibraryItemOf(paperID string) (LibraryItem, Response) {
	s.t.Helper()
	resp := s.Get("/library/" + paperID)
	var it LibraryItem
	if resp.Status == 200 {
		resp.JSON(s.t, &it)
	}
	return it, resp
}

// PatchLibrary changes status / favorite / folder of a library row.
func (s *Session) PatchLibrary(paperID string, body map[string]any) (LibraryItem, Response) {
	s.t.Helper()
	resp := s.Patch("/library/"+paperID, body)
	var it LibraryItem
	if resp.Status == 200 {
		resp.JSON(s.t, &it)
	}
	return it, resp
}

// RemoveFromLibrary deletes the user's own link to a paper.
func (s *Session) RemoveFromLibrary(paperID string) Response {
	s.t.Helper()
	return s.Delete("/library/" + paperID)
}

// LibraryWhere lists the library with a raw query string (e.g. "status=read").
func (s *Session) LibraryWhere(query string) ([]LibraryItem, int, Response) {
	s.t.Helper()
	resp := s.Get("/library?limit=100&" + query)
	var out struct {
		Items []LibraryItem `json:"items"`
		Total int           `json:"total"`
	}
	if resp.Status == 200 {
		resp.JSON(s.t, &out)
	}
	return out.Items, out.Total, resp
}
