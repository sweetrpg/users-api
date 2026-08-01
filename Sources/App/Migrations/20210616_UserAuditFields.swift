//
// Created by Paul Schifferer on 6/16/21.
//

// import Fluent

// struct UserAuditFields : Migration {
//     func prepare(on database : Database) -> EventLoopFuture<()> {
//         database.schema(User.v20210601.schemaName)
//                 .field(User.v20210616.createdAt, .datetime, .required)
//                 .field(User.v20210616.updatedAt, .datetime, .required)
//                 .update()
//     }

//     func revert(on database : Database) -> EventLoopFuture<()> {
//         database.schema(User.v20210601.schemaName)
//                 .deleteField(User.v20210616.createdAt)
//                 .deleteField(User.v20210616.updatedAt)
//                 .update()
//     }
// }
