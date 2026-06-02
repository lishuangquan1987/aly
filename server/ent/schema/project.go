package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Project holds the schema definition for the Project entity.
type Project struct {
	ent.Schema
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Unique().Comment("项目名称，用于创建文件夹保存文件，创建后不能更改"),
		field.String("title").Comment("项目抬头"),
		field.String("version").Comment("项目版本"),
		field.Bool("force_update").Comment("是否强制更新"),
		field.JSON("ignore_folders", []string{}).Optional().Comment("忽略的文件夹"),
		field.JSON("ignore_files", []string{}).Optional().Comment("忽略的文件"),
		field.Time("created_at").Default(time.Now).Comment("创建日期"),
		field.Bool("is_deleted").Default(false).Comment("是否被删除"),
	}
}

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("change_logs", ProjectChangeLog.Type),
	}
}
