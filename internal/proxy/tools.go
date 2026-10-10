package proxy

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addTool[T any](server *mcp.Server, c *Client, name, title, desc string) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Title: title, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args T) (*mcp.CallToolResult, any, error) {
			return c.handleTool(ctx, name, args)
		})
}

type pingArgs struct{}

type getTreeArgs struct {
	FolderID *string  `json:"folder_id,omitempty" jsonschema:"Folder ID to get subtree of. Omit for full tree."`
	Depth    *float64 `json:"depth,omitempty" jsonschema:"Max depth to return (default: 2). Folders beyond this show childCount instead of children. Use 0 for unlimited."`
}

type listArgs struct {
	FolderID *string  `json:"folder_id,omitempty" jsonschema:"Folder ID to list. Default: root (0)."`
	Limit    *float64 `json:"limit,omitempty" jsonschema:"Max items to return. Default: 100."`
	Offset   *float64 `json:"offset,omitempty" jsonschema:"Number of items to skip. Default: 0."`
}

type searchArgs struct {
	Query      string   `json:"query" jsonschema:"Search query to match against titles and URLs."`
	FolderID   *string  `json:"folder_id,omitempty" jsonschema:"Scope search to a specific folder subtree by ID."`
	FolderPath *string  `json:"folder_path,omitempty" jsonschema:"Scope search by folder path, e.g. \"Bookmarks Bar > Tech > AI\". Alternative to folder_id."`
	Limit      *float64 `json:"limit,omitempty" jsonschema:"Max results to return. Default: 50."`
}

type getArgs struct {
	ID string `json:"id" jsonschema:"Bookmark or folder ID."`
}

type countArgs struct {
	FolderID *string `json:"folder_id,omitempty" jsonschema:"Folder ID to count within. Omit for global count."`
}

type findDuplicatesArgs struct {
	FolderID *string `json:"folder_id,omitempty" jsonschema:"Scope duplicate search to a specific folder."`
}

type createArgs struct {
	Title         string   `json:"title" jsonschema:"Title of the bookmark or folder."`
	URL           *string  `json:"url,omitempty" jsonschema:"URL for a bookmark. Omit to create a folder."`
	ParentID      *string  `json:"parent_id,omitempty" jsonschema:"Parent folder ID. Default: Bookmarks Bar (1)."`
	ParentPath    *string  `json:"parent_path,omitempty" jsonschema:"Parent folder path like \"Tech > AI\". Used with create_parents."`
	CreateParents *bool    `json:"create_parents,omitempty" jsonschema:"Auto-create parent folders from parent_path if they don't exist."`
	Index         *float64 `json:"index,omitempty" jsonschema:"Position within the parent folder."`
}

type updateArgs struct {
	ID    string  `json:"id" jsonschema:"Bookmark or folder ID to update."`
	Title *string `json:"title,omitempty" jsonschema:"New title."`
	URL   *string `json:"url,omitempty" jsonschema:"New URL (bookmarks only)."`
}

type moveArgs struct {
	ID       string   `json:"id" jsonschema:"Bookmark or folder ID to move."`
	ParentID string   `json:"parent_id" jsonschema:"Target parent folder ID."`
	Index    *float64 `json:"index,omitempty" jsonschema:"Position within the target folder."`
}

type deleteArgs struct {
	ID string `json:"id" jsonschema:"Bookmark ID to delete."`
}

type deleteFolderArgs struct {
	ID      string `json:"id" jsonschema:"Folder ID to delete."`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to confirm deletion of folder and all contents."`
}

type batchMoveArgs struct {
	IDs      []string `json:"ids" jsonschema:"Array of bookmark IDs to move."`
	ParentID string   `json:"parent_id" jsonschema:"Target parent folder ID."`
}

type mergeFoldersArgs struct {
	SourceID     string `json:"source_id" jsonschema:"Source folder ID to merge from."`
	TargetID     string `json:"target_id" jsonschema:"Target folder ID to merge into."`
	DeleteSource *bool  `json:"delete_source,omitempty" jsonschema:"Delete source folder after merge. Default: false."`
	Deduplicate  *bool  `json:"deduplicate,omitempty" jsonschema:"Skip moving bookmarks that already exist in target (by URL). Default: false."`
	Confirm      *bool  `json:"confirm,omitempty" jsonschema:"Required (with dry_run unset/false) to actually delete the source folder and/or duplicate bookmarks."`
	DryRun       *bool  `json:"dry_run,omitempty" jsonschema:"Preview only: report what would be removed without deleting anything."`
}

type deduplicateArgs struct {
	FolderID *string `json:"folder_id,omitempty" jsonschema:"Scope to a specific folder. Required unless scope:\"global\" is set."`
	Scope    *string `json:"scope,omitempty" jsonschema:"Set to \"global\" to deduplicate across the entire bookmark tree. Required when folder_id is omitted."`
	Keep     *string `json:"keep,omitempty" jsonschema:"Which duplicate to keep: first or last. Default: first."`
	Confirm  *bool   `json:"confirm,omitempty" jsonschema:"Required (with dry_run unset/false) to actually delete the duplicates."`
	DryRun   *bool   `json:"dry_run,omitempty" jsonschema:"Preview only: report which duplicates would be removed without deleting anything."`
}

type batchDeleteArgs struct {
	IDs     []string `json:"ids" jsonschema:"Array of bookmark IDs to delete."`
	Confirm *bool    `json:"confirm,omitempty" jsonschema:"Required (with dry_run unset/false) to actually delete the bookmarks."`
	DryRun  *bool    `json:"dry_run,omitempty" jsonschema:"Preview only: report which bookmarks would be removed without deleting anything."`
}

type exportHTMLArgs struct {
	FolderID *string `json:"folder_id,omitempty" jsonschema:"Export a specific folder subtree. Omit for all bookmarks."`
}

type importHTMLArgs struct {
	HTML     string  `json:"html" jsonschema:"Netscape Bookmark HTML content to import."`
	ParentID *string `json:"parent_id,omitempty" jsonschema:"Parent folder ID to import into. Default: Bookmarks Bar (1)."`
}

type checkDeadLinksArgs struct {
	FolderID  *string  `json:"folder_id,omitempty" jsonschema:"Scope check to a specific folder."`
	Limit     *float64 `json:"limit,omitempty" jsonschema:"Max bookmarks to check. Default: 50."`
	TimeoutMS *float64 `json:"timeout_ms,omitempty" jsonschema:"HTTP request timeout in ms. Default: 5000."`
}

func addTools(server *mcp.Server, c *Client) {
	addTool[pingArgs](server, c, "ping", "Ping", "Check if the browser extension is connected and responsive.")
	addTool[getTreeArgs](server, c, "bookmark_get_tree", "Get Bookmark Tree", "Returns the bookmark tree structure. Default depth is 2 to stay within MCP token limits. Use higher depth for more detail, or depth: 0 for unlimited. Use folder_id to get a subtree.")
	addTool[listArgs](server, c, "bookmark_list", "List Bookmarks", "List bookmarks in a specific folder (non-recursive). Returns items with folder paths.")
	addTool[searchArgs](server, c, "bookmark_search", "Search Bookmarks", "Full-text search across bookmark titles and URLs. Scope to a folder by ID or path.")
	addTool[getArgs](server, c, "bookmark_get", "Get Bookmark", "Get a single bookmark or folder by ID with full details including folder path.")
	addTool[countArgs](server, c, "bookmark_count", "Count Bookmarks", "Count total bookmarks and folders, optionally scoped to a specific folder.")
	addTool[findDuplicatesArgs](server, c, "bookmark_find_duplicates", "Find Duplicate Bookmarks", "Find bookmarks with duplicate URLs. Returns groups of duplicates with their locations.")
	addTool[createArgs](server, c, "bookmark_create", "Create Bookmark", "Create a new bookmark or folder. Omit url to create a folder. Set create_parents: true with parent_path to auto-create nested folders.")
	addTool[updateArgs](server, c, "bookmark_update", "Update Bookmark", "Update the title or URL of an existing bookmark or folder.")
	addTool[moveArgs](server, c, "bookmark_move", "Move Bookmark", "Move a bookmark or folder to a different parent folder.")
	addTool[deleteArgs](server, c, "bookmark_delete", "Delete Bookmark", "Delete a single bookmark by ID. Cannot delete root folders.")
	addTool[deleteFolderArgs](server, c, "bookmark_delete_folder", "Delete Folder", "Delete a folder and ALL its contents. Requires confirm: true as a safety gate. Permanently deletes the folder and every bookmark inside it, with no undo. Back up first with bookmark_export_html.")
	addTool[batchMoveArgs](server, c, "bookmark_batch_move", "Batch Move Bookmarks", "Move multiple bookmarks to a target folder at once.")
	addTool[mergeFoldersArgs](server, c, "bookmark_merge_folders", "Merge Folders", "Merge all contents of source folder into target folder. Optionally deduplicate and delete source. When delete_source is true, or deduplicate removes duplicates, this permanently deletes bookmarks, with no undo: call with dry_run:true first to preview what would be removed, then confirm:true to execute. Back up first with bookmark_export_html.")
	addTool[deduplicateArgs](server, c, "bookmark_deduplicate", "Deduplicate Bookmarks", "Find and remove duplicate bookmarks (same URL) within a folder or globally. Permanently deletes the redundant copies, with no undo: call with dry_run:true first to preview which bookmarks would be removed, then confirm:true to execute. You must pass folder_id or scope:\"global\" so an omitted scope can never wipe the whole tree. Back up first with bookmark_export_html.")
	addTool[batchDeleteArgs](server, c, "bookmark_batch_delete", "Batch Delete Bookmarks", "Delete multiple bookmarks by their IDs. Cannot delete root folders. Permanently deletes the listed bookmarks, with no undo: call with dry_run:true first to preview the items, then confirm:true to execute. Back up first with bookmark_export_html.")
	addTool[exportHTMLArgs](server, c, "bookmark_export_html", "Export Bookmarks as HTML", "Export bookmarks in Netscape Bookmark HTML format (same as browser export). Returns the HTML string.")
	addTool[importHTMLArgs](server, c, "bookmark_import_html", "Import Bookmarks from HTML", "Import bookmarks from Netscape Bookmark HTML format into the browser.")
	addTool[checkDeadLinksArgs](server, c, "bookmark_check_dead_links", "Check Dead Links", "Check bookmarks for broken/dead URLs by making HTTP requests. Checks in batches of 5.")
}
