//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F05 — библиотека и папки. Ист.: 006_library_folders.sql, library/store.go, library/http.go, iteration-1.

// publicPaper opens a catalog paper without touching the user's library.
func publicPaper(t *testing.T, s *client.Session, arxivID string) client.Paper {
	t.Helper()
	p, resp := s.OpenArxiv(arxivID)
	require.Contains(t, []int{200, 201}, resp.Status, resp)
	return p
}

// F05-01 У нового пользователя три системные папки в фиксированном порядке.
func TestF05_SystemFolders(t *testing.T) {
	u := user(t, "f05-system")
	folders, resp := u.Folders()
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, folders, 3)
	var keys, names []string
	for _, f := range folders {
		require.NotNil(t, f.SystemKey)
		keys = append(keys, *f.SystemKey)
		names = append(names, f.Name)
		require.Equal(t, 0, f.ArticleCount)
		require.Nil(t, f.ParentID)
	}
	require.Equal(t, []string{"want_to_read", "reading", "other"}, keys)
	require.Equal(t, []string{"Хочу прочитать", "Читаю сейчас", "Другое"}, names)
	again, _ := u.Folders()
	require.Len(t, again, 3, "system folders are created once")
}

// F05-02 + F05-03 Сохранение статьи: по умолчанию в «Хочу прочитать», можно сразу в свою папку.
func TestF05_SaveToLibrary(t *testing.T) {
	u := user(t, "f05-save")
	paper := publicPaper(t, u.Session, "2301.00001")
	item, resp := u.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, "unread", item.Status)
	require.False(t, item.Favorite)
	want := u.SystemFolder("want_to_read")
	require.NotNil(t, item.FolderID)
	require.Equal(t, want.ID, *item.FolderID)
	require.Equal(t, 1, want.ArticleCount)

	again, resp := u.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, item.ID, again.ID, "saving twice keeps one row")

	folder, resp := u.CreateFolder("Thesis", "")
	require.Equal(t, 201, resp.Status, resp)
	other := publicPaper(t, u.Session, "2301.00002")
	inFolder, resp := u.AddToLibrary(other.ID, folder.ID)
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, folder.ID, *inFolder.FolderID)

	_, resp = u.AddToLibrary(publicPaper(t, u.Session, "2301.00003").ID, "00000000-0000-0000-0000-000000000001")
	require.Equal(t, 404, resp.Status, resp)
}

// F05-04 Создание папок: дубль, вложенность, плохие родители и имена.
func TestF05_CreateFolders(t *testing.T) {
	u := user(t, "f05-folders")
	parent, resp := u.CreateFolder("Reading group", "")
	require.Equal(t, 201, resp.Status, resp)
	require.Nil(t, parent.ParentID)
	require.Nil(t, parent.SystemKey)

	_, resp = u.CreateFolder("reading GROUP", "")
	require.Equal(t, 409, resp.Status, "same name (case-insensitive) at the same level: %s", resp)

	child, resp := u.CreateFolder("Reading group", parent.ID)
	require.Equal(t, 201, resp.Status, "the same name under another parent is fine: %s", resp)
	require.NotNil(t, child.ParentID)
	require.Equal(t, parent.ID, *child.ParentID)

	_, resp = u.CreateFolder("Orphan", "00000000-0000-0000-0000-000000000001")
	require.Equal(t, 404, resp.Status, resp)
	stranger := user(t, "f05-stranger")
	_, resp = stranger.CreateFolder("Sneaky", parent.ID)
	require.Equal(t, 404, resp.Status, "another user's folder is not a valid parent: %s", resp)

	_, resp = u.CreateFolder("   ", "")
	require.Equal(t, 400, resp.Status, resp)
	_, resp = u.CreateFolder(strings.Repeat("x", 121), "")
	require.Equal(t, 400, resp.Status, resp)
	folder120, resp := u.CreateFolder(strings.Repeat("y", 120), "")
	require.Equal(t, 201, resp.Status, resp)
	require.Len(t, []rune(folder120.Name), 120)

	folders, _ := u.Folders()
	require.Len(t, folders, 3+3, "three system folders plus three own")
}

// F05-05 + F05-06 Перенос в системную папку меняет статус; статус и избранное меняются напрямую.
func TestF05_MoveAndStatus(t *testing.T) {
	u := user(t, "f05-move")
	paper := publicPaper(t, u.Session, "2301.00001")
	_, resp := u.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	own, _ := u.CreateFolder("Own", "")

	moved, resp := u.PatchLibrary(paper.ID, map[string]any{"folder_id": own.ID})
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, own.ID, *moved.FolderID)
	require.Equal(t, "unread", moved.Status, "an own folder does not change the status")

	for key, status := range map[string]string{"reading": "reading", "other": "read", "want_to_read": "unread"} {
		folder := u.SystemFolder(key)
		moved, resp := u.PatchLibrary(paper.ID, map[string]any{"folder_id": folder.ID})
		require.Equal(t, 200, resp.Status, resp)
		require.Equal(t, folder.ID, *moved.FolderID)
		require.Equal(t, status, moved.Status, "moving into %s sets status %s", key, status)
	}

	for _, status := range []string{"reading", "read", "unread"} {
		it, resp := u.PatchLibrary(paper.ID, map[string]any{"status": status})
		require.Equal(t, 200, resp.Status, resp)
		require.Equal(t, status, it.Status)
	}
	_, resp = u.PatchLibrary(paper.ID, map[string]any{"status": "bogus"})
	require.Equal(t, 400, resp.Status, resp)

	fav, resp := u.PatchLibrary(paper.ID, map[string]any{"favorite": true})
	require.Equal(t, 200, resp.Status, resp)
	require.True(t, fav.Favorite)
	it, _ := u.LibraryItemOf(paper.ID)
	require.True(t, it.Favorite)

	_, resp = u.PatchLibrary("00000000-0000-0000-0000-000000000001", map[string]any{"favorite": true})
	require.Equal(t, 404, resp.Status, resp)
}

// F05-07 Удаление папки переносит статьи в «Другое» как прочитанные; системные папки неудаляемы.
func TestF05_DeleteFolder(t *testing.T) {
	u := user(t, "f05-delfolder")
	folder, _ := u.CreateFolder("Temporary", "")
	paper := publicPaper(t, u.Session, "2301.00002")
	_, resp := u.AddToLibrary(paper.ID, folder.ID)
	require.Equal(t, 201, resp.Status, resp)

	require.Equal(t, 204, u.DeleteFolder(folder.ID).Status)
	it, resp := u.LibraryItemOf(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, u.SystemFolder("other").ID, *it.FolderID, "papers of a deleted folder land in «Другое»")
	require.Equal(t, "read", it.Status)
	folders, _ := u.Folders()
	for _, f := range folders {
		require.NotEqual(t, folder.ID, f.ID, "the folder is gone")
	}

	require.Equal(t, 404, u.DeleteFolder(folder.ID).Status)
	resp = u.DeleteFolder(u.SystemFolder("reading").ID)
	require.Equal(t, 400, resp.Status, resp)
	require.Equal(t, 404, u.DeleteFolder("00000000-0000-0000-0000-000000000001").Status)
}

// F05-08 Удаление статьи из библиотеки трогает только свою ссылку.
func TestF05_RemoveFromLibrary(t *testing.T) {
	a := user(t, "f05-rm-a")
	b := user(t, "f05-rm-b")
	paper := publicPaper(t, a.Session, "2301.00003")
	_, resp := a.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	_, resp = b.AddToLibrary(paper.ID, "")
	require.Equal(t, 201, resp.Status, resp)

	require.Equal(t, 204, a.RemoveFromLibrary(paper.ID).Status)
	require.False(t, a.HasInLibrary(paper.ID))
	require.True(t, b.HasInLibrary(paper.ID), "the other user's link is intact")
	require.Equal(t, 200, a.Get("/papers/"+paper.ID).Status, "a public paper stays readable")
	require.Equal(t, 404, a.RemoveFromLibrary(paper.ID).Status)
	require.Equal(t, 404, a.RemoveFromLibrary("not-a-uuid").Status)
}

// F05-09 + F05-10 Фильтры и пагинация списка.
func TestF05_ListFiltersAndPaging(t *testing.T) {
	u := user(t, "f05-list")
	own, _ := u.CreateFolder("Own", "")
	p1 := publicPaper(t, u.Session, "2301.00001")
	p2 := publicPaper(t, u.Session, "2301.00002")
	p3 := publicPaper(t, u.Session, "2301.00003")
	_, resp := u.AddToLibrary(p1.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	_, resp = u.AddToLibrary(p2.ID, own.ID)
	require.Equal(t, 201, resp.Status, resp)
	_, resp = u.AddToLibrary(p3.ID, "")
	require.Equal(t, 201, resp.Status, resp)
	_, resp = u.PatchLibrary(p3.ID, map[string]any{"status": "read"})
	require.Equal(t, 200, resp.Status, resp)

	items, total, resp := u.LibraryWhere("status=read")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, p3.ID, items[0].Paper.ID)

	items, total, resp = u.LibraryWhere("folder_id=" + own.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, 1, total)
	require.Equal(t, p2.ID, items[0].Paper.ID)

	_, _, resp = u.LibraryWhere("folder_id=abc")
	require.Equal(t, 400, resp.Status, resp)
	// D-05: the status filter is not validated (unknown status → empty list instead of 400).
	_, _, resp = u.LibraryWhere("status=bogus")
	require.Equal(t, 400, resp.Status, "an unknown status filter is a client error, like in PATCH: %s", resp)

	page1 := u.Get("/library?limit=2&page=1").Map(t)
	require.EqualValues(t, 3, page1["total"])
	require.Len(t, page1["items"].([]any), 2)
	page2 := u.Get("/library?limit=2&page=2").Map(t)
	require.Len(t, page2["items"].([]any), 1)
	require.EqualValues(t, 20, u.Get("/library?limit=0").Map(t)["limit"], "default page size")
	require.EqualValues(t, 100, u.Get("/library?limit=1000").Map(t)["limit"], "page size is capped")
}

// F05-11 Чужая загрузка и несуществующие статьи не сохраняются.
func TestF05_ForeignAndUnknownPapers(t *testing.T) {
	owner := user(t, "f05-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	u := user(t, "f05-foreign")
	_, resp = u.AddToLibrary(private.ID, "")
	require.Equal(t, 404, resp.Status, "another user's upload cannot be saved: %s", resp)
	_, resp = u.AddToLibrary("00000000-0000-0000-0000-000000000001", "")
	require.Equal(t, 404, resp.Status, resp)
	_, resp = u.LibraryItemOf("00000000-0000-0000-0000-000000000001")
	require.Equal(t, 404, resp.Status, resp)
	_, resp = u.LibraryItemOf("not-a-uuid")
	require.Equal(t, 404, resp.Status, resp)
}

// F05-12 [assumption] Кнопка «Сохранить» без тела запроса — D-04.
func TestF05_SaveWithoutBody(t *testing.T) {
	u := user(t, "f05-nobody")
	paper := publicPaper(t, u.Session, "2301.00004")
	resp := u.Post("/library/"+paper.ID, nil)
	require.Equal(t, 201, resp.Status, "saving without a body must work like saving with {}: %s", resp)
}

// F05-13 Мусор в теле → 400.
func TestF05_MalformedBodies(t *testing.T) {
	u := user(t, "f05-junk")
	paper := publicPaper(t, u.Session, "2301.00005")
	require.Equal(t, 400, u.Post("/library/"+paper.ID, map[string]any{"surprise": 1}).Status)
	require.Equal(t, 400, u.Post("/library/"+paper.ID, "{not json").Status)
	require.Equal(t, 400, u.Post("/library/folders", map[string]any{"name": "x", "color": "red"}).Status)
	require.Equal(t, 400, u.Patch("/library/"+paper.ID, map[string]any{"status": "read", "extra": true}).Status)
}
