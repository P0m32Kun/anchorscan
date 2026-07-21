package web

import (
	"net/http"
	"os"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/preflight"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

func (s *server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, _ := s.store.ListProjects()
	runs, _ := s.store.ListScanRuns(10)
	tmpl, err := parseTemplates("templates/home.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = tmpl.ExecuteTemplate(w, "base", map[string]any{
		"Projects": projects,
		"Runs":     runs,
	})
}

func (s *server) projects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects, err := s.store.ListProjects()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, "templates/projects.html", map[string]any{"Projects": projects})
	case http.MethodPost:
		if err := parseProjectRequest(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		now := s.opts.Now()
		project := store.Project{
			ID:          newID("project", now),
			Name:        name,
			Description: r.FormValue("description"),
			ClientUnit:  strings.TrimSpace(r.FormValue("client_unit")),
			ReportTitle: strings.TrimSpace(r.FormValue("report_title")),
			TestObject:  strings.TrimSpace(r.FormValue("test_object")),
			StartDate:   strings.TrimSpace(r.FormValue("start_date")),
			EndDate:     strings.TrimSpace(r.FormValue("end_date")),
			Testers:     strings.TrimSpace(r.FormValue("testers")),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if project.ReportTitle == "" && project.ClientUnit != "" {
			project.ReportTitle = project.ClientUnit + "内网安全检查报告"
		}
		if err := s.store.SaveProject(project); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.store.CreateDefaultProjectZones(project.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects/"+project.ID, http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) projectNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	render(w, "templates/project_form.html", map[string]any{
		"Title":   "新建项目",
		"Action":  "/projects",
		"Project": store.Project{},
	})
}

func (s *server) projectDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/projects/")
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := segments[0]

	// Sub-routes: /projects/{id}/scans/new, /projects/{id}/edit, /projects/{id}/zones, /projects/{id}/zones/{zoneID}
	switch {
	case r.Method == http.MethodGet && len(segments) == 3 && segments[1] == "scans" && segments[2] == "new":
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		zones, err := s.store.ListProjectZones(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		form := scanForm{}
		if rerunID := strings.TrimSpace(r.URL.Query().Get("rerun")); rerunID != "" {
			form, err = s.rerunScanForm(id, rerunID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		s.renderProjectScanForm(w, project, zones, preflight.Result{}, form)
	case r.Method == http.MethodGet && len(segments) == 2 && segments[1] == "edit":
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		render(w, "templates/project_form.html", map[string]any{
			"Title":   "编辑项目",
			"Action":  "/projects/" + id,
			"Project": project,
		})
	case r.Method == http.MethodPost && len(segments) >= 2 && segments[1] == "zones":
		s.projectZones(w, r, id, segments)
	case r.Method == http.MethodGet && len(segments) == 2 && segments[1] == "workbench":
		s.projectWorkbench(w, r, id)
	case r.Method == http.MethodPost && len(segments) == 4 && segments[1] == "candidates" && segments[3] == "commands":
		s.projectCandidateCommand(w, r, id, segments[2])
	case len(segments) >= 2 && segments[1] == "verifications":
		s.projectVerifications(w, r, id, segments)
	case r.Method == http.MethodGet:
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		zones, err := s.store.ListProjectZones(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		runs, err := s.store.ListProjectScanRuns(id, 100)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		zoneNames := make(map[string]string, len(zones))
		for _, z := range zones {
			zoneNames[z.ZoneID] = z.Name
		}
		render(w, "templates/project_detail.html", map[string]any{
			"Title":     project.Name,
			"Project":   project,
			"Zones":     zones,
			"ZoneNames": zoneNames,
			"Runs":      runs,
		})
	case r.Method == http.MethodPost:
		if err := parseProjectRequest(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.FormValue("_method") == "delete" {
			hasRunning, err := s.store.ProjectHasRunningRuns(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if hasRunning {
				http.Error(w, "project has running scans", http.StatusConflict)
				return
			}
			artifactDirs, err := s.store.ListProjectArtifactDirs(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := s.store.DeleteProjectCascade(id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for _, dir := range artifactDirs {
				if strings.TrimSpace(dir) == "" {
					continue
				}
				if err := os.RemoveAll(dir); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
			if err := os.RemoveAll(managedProjectDir(s.opts.DBPath, id)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/projects", http.StatusSeeOther)
			return
		}
		project, err := s.store.GetProject(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		project.Name = strings.TrimSpace(r.FormValue("name"))
		project.Description = r.FormValue("description")
		project.ClientUnit = strings.TrimSpace(r.FormValue("client_unit"))
		project.ReportTitle = strings.TrimSpace(r.FormValue("report_title"))
		project.TestObject = strings.TrimSpace(r.FormValue("test_object"))
		project.StartDate = strings.TrimSpace(r.FormValue("start_date"))
		project.EndDate = strings.TrimSpace(r.FormValue("end_date"))
		project.Testers = strings.TrimSpace(r.FormValue("testers"))
		if project.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if project.ReportTitle == "" && project.ClientUnit != "" {
			project.ReportTitle = project.ClientUnit + "内网安全检查报告"
		}
		project.UpdatedAt = s.opts.Now()
		if err := s.store.SaveProject(project); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *server) projectZones(w http.ResponseWriter, r *http.Request, projectID string, segments []string) {
	if _, err := s.store.GetProject(projectID); err != nil {
		http.NotFound(w, r)
		return
	}

	if len(segments) == 2 {
		// Add zone: POST /projects/{id}/zones
		if err := parseProjectRequest(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "zone name is required", http.StatusBadRequest)
			return
		}
		sortOrder, err := s.store.NextProjectZoneSortOrder(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		zone := store.ProjectZone{
			ProjectID: projectID,
			ZoneID:    newID("zone", s.opts.Now()),
			Name:      name,
			SortOrder: sortOrder,
		}
		if err := s.store.CreateProjectZone(zone); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects/"+projectID, http.StatusSeeOther)
		return
	}

	if len(segments) == 3 && r.FormValue("_method") == "delete" {
		// Delete zone: POST /projects/{id}/zones/{zoneID}
		zoneID := segments[2]
		hasRuns, err := s.store.ZoneHasRuns(projectID, zoneID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if hasRuns {
			http.Error(w, "zone has runs and cannot be deleted", http.StatusConflict)
			return
		}
		hasVerifications, err := s.store.ZoneHasVerifications(projectID, zoneID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if hasVerifications {
			http.Error(w, "zone has verifications and cannot be deleted", http.StatusConflict)
			return
		}
		if err := s.store.DeleteProjectZone(projectID, zoneID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/projects/"+projectID, http.StatusSeeOther)
		return
	}

	http.NotFound(w, r)
}

func parseProjectRequest(r *http.Request) error {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		return r.ParseMultipartForm(8 << 20)
	}
	return r.ParseForm()
}
