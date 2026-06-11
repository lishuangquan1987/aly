package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// ProjectChangeLog holds the schema definition for the ProjectChangeLog entity.
type ProjectChangeLog struct {
	ent.Schema
}

// Fields of the ProjectChangeLog.
func (ProjectChangeLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("version").Comment("版本号"),
		field.JSON("logs", []string{}).Comment("变更日志集合"),
		field.String("time").Comment("变更时间"),
		field.Time("created_at").Default(time.Now).Comment("创建日期"),
		field.Bool("is_deleted").Default(false).Comment("是否被删除"),
		field.String("after_apply_update_script").Optional().Comment("更新后执行的脚本（相对于应用目录）"),
	}
}

// Edges of the ProjectChangeLog.
func (ProjectChangeLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("project", Project.Type).Ref("change_logs").Unique(),
	}
}
