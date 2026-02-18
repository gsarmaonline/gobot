package models

import (
	"database/sql"
	"time"
)

type HierarchyRelation struct {
	ID           string    `json:"id"`
	OrgID        string    `json:"orgId"`
	ParentType   string    `json:"parentType"` // "user" | "agent"
	ParentID     string    `json:"parentId"`
	ChildAgentID string    `json:"childAgentId"`
	Relationship string    `json:"relationship"` // "reports_to" | "delegates_to" | "escalates_to"
	CreatedAt    time.Time `json:"createdAt"`
}

// HierarchyNode represents a node in the org tree for API responses
type HierarchyNode struct {
	Type     string          `json:"type"` // "user" | "agent"
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Title    string          `json:"title,omitempty"`
	Status   string          `json:"status,omitempty"` // only for agents
	Children []HierarchyNode `json:"children,omitempty"`
}

type HierarchyStore struct {
	db *sql.DB
}

func NewHierarchyStore(db *sql.DB) *HierarchyStore {
	return &HierarchyStore{db: db}
}

func (s *HierarchyStore) Create(rel *HierarchyRelation) error {
	return s.db.QueryRow(
		`INSERT INTO agent_hierarchy (org_id, parent_type, parent_id, child_agent_id, relationship)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		rel.OrgID, rel.ParentType, rel.ParentID, rel.ChildAgentID, rel.Relationship,
	).Scan(&rel.ID, &rel.CreatedAt)
}

func (s *HierarchyStore) GetByOrg(orgID string) ([]HierarchyRelation, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, parent_type, parent_id, child_agent_id, relationship, created_at
		 FROM agent_hierarchy WHERE org_id = $1 ORDER BY created_at`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []HierarchyRelation
	for rows.Next() {
		var rel HierarchyRelation
		if err := rows.Scan(&rel.ID, &rel.OrgID, &rel.ParentType, &rel.ParentID,
			&rel.ChildAgentID, &rel.Relationship, &rel.CreatedAt); err != nil {
			return nil, err
		}
		relations = append(relations, rel)
	}
	return relations, rows.Err()
}

func (s *HierarchyStore) GetChildrenOf(parentType, parentID string) ([]HierarchyRelation, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, parent_type, parent_id, child_agent_id, relationship, created_at
		 FROM agent_hierarchy WHERE parent_type = $1 AND parent_id = $2`, parentType, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []HierarchyRelation
	for rows.Next() {
		var rel HierarchyRelation
		if err := rows.Scan(&rel.ID, &rel.OrgID, &rel.ParentType, &rel.ParentID,
			&rel.ChildAgentID, &rel.Relationship, &rel.CreatedAt); err != nil {
			return nil, err
		}
		relations = append(relations, rel)
	}
	return relations, rows.Err()
}

func (s *HierarchyStore) GetParentOf(childAgentID string) (*HierarchyRelation, error) {
	rel := &HierarchyRelation{}
	err := s.db.QueryRow(
		`SELECT id, org_id, parent_type, parent_id, child_agent_id, relationship, created_at
		 FROM agent_hierarchy WHERE child_agent_id = $1 AND relationship = 'reports_to'`,
		childAgentID,
	).Scan(&rel.ID, &rel.OrgID, &rel.ParentType, &rel.ParentID,
		&rel.ChildAgentID, &rel.Relationship, &rel.CreatedAt)
	if err != nil {
		return nil, err
	}
	return rel, nil
}

func (s *HierarchyStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM agent_hierarchy WHERE id = $1`, id)
	return err
}

func (s *HierarchyStore) DeleteByChild(childAgentID string) error {
	_, err := s.db.Exec(`DELETE FROM agent_hierarchy WHERE child_agent_id = $1`, childAgentID)
	return err
}

// BuildTree constructs the org hierarchy tree for an organization
func (s *HierarchyStore) BuildTree(orgID string) ([]HierarchyNode, error) {
	// Get all users (potential roots)
	userRows, err := s.db.Query(
		`SELECT u.id, u.name, u.role FROM users u WHERE u.org_id = $1 ORDER BY u.name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer userRows.Close()

	// Get all hierarchy relations
	relations, err := s.GetByOrg(orgID)
	if err != nil {
		return nil, err
	}

	// Get all agents
	agentRows, err := s.db.Query(
		`SELECT id, name, COALESCE(title, ''), status FROM agents WHERE org_id = $1`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer agentRows.Close()

	// Build lookup maps
	agentMap := make(map[string]HierarchyNode)
	for agentRows.Next() {
		var node HierarchyNode
		if err := agentRows.Scan(&node.ID, &node.Name, &node.Title, &node.Status); err != nil {
			return nil, err
		}
		node.Type = "agent"
		agentMap[node.ID] = node
	}

	// Build parent -> children map
	childrenMap := make(map[string][]string) // parentKey -> []childAgentID
	childHasParent := make(map[string]bool)
	for _, rel := range relations {
		if rel.Relationship == "reports_to" {
			key := rel.ParentType + ":" + rel.ParentID
			childrenMap[key] = append(childrenMap[key], rel.ChildAgentID)
			childHasParent[rel.ChildAgentID] = true
		}
	}

	// Build user nodes with their children
	var roots []HierarchyNode
	for userRows.Next() {
		var node HierarchyNode
		var role string
		if err := userRows.Scan(&node.ID, &node.Name, &role); err != nil {
			return nil, err
		}
		node.Type = "user"
		node.Title = role
		node.Children = s.buildChildren("user", node.ID, childrenMap, agentMap)
		roots = append(roots, node)
	}

	// Add orphan agents (no parent) as root nodes
	for id, agent := range agentMap {
		if !childHasParent[id] {
			agent.Children = s.buildChildren("agent", id, childrenMap, agentMap)
			roots = append(roots, agent)
		}
	}

	return roots, nil
}

func (s *HierarchyStore) buildChildren(parentType, parentID string, childrenMap map[string][]string, agentMap map[string]HierarchyNode) []HierarchyNode {
	key := parentType + ":" + parentID
	childIDs := childrenMap[key]
	if len(childIDs) == 0 {
		return nil
	}

	var children []HierarchyNode
	for _, childID := range childIDs {
		if agent, ok := agentMap[childID]; ok {
			agent.Children = s.buildChildren("agent", childID, childrenMap, agentMap)
			children = append(children, agent)
		}
	}
	return children
}
