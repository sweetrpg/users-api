//
// Created by Paul Schifferer on 6/16/21.
//

// import Fluent

// struct SoftDeleteUser : Migration {
//     func prepare(on database : Database) -> EventLoopFuture<()> {
//         database.schema(User.v20210601.schemaName)
//                 .field(User.v20210616.deletedAt, .datetime)
//                 .update()
//     }

//     func revert(on database : Database) -> EventLoopFuture<()> {
//         database.schema(User.v20210601.schemaName)
//                 .deleteField(User.v20210616.deletedAt)
//                 .update()
//     }
// }
