//
// Token.swift
// Copyright (c) 2021 Paul Schifferer.
//

// import Fluent
// import Vapor

// final class Token : Model {
//     static let schema = Token.v20210601.schemaName

//     @ID
//     var id : UUID?

//     @Timestamp(key: Token.v20210616.createdAt, on: .create)
//     var createdAt : Date?

//     @Timestamp(key: Token.v20210616.updatedAt, on: .update)
//     var updatedAt : Date?

//     @Field(key: Token.v20210601.value)
//     var value : String

//     @Parent(key: Token.v20210601.userId)
//     var user : User

//     init() {
//     }

//     init(id : UUID? = nil, value : String, userId : User.IDValue) {
//         self.id = id
//         self.value = value
//         $user.id = userId
//     }
// }

// extension Token : Content {}

// extension Token {
//     static func generate(for user : User) throws -> Token {
//         let random = [ UInt8 ].random(count: 56).base64
//         return try Token(value: random, userId: user.requireID())
//     }
// }

// extension Token : ModelTokenAuthenticatable {
//     static let valueKey = \Token.$value
//     static let userKey = \Token.$user
//     typealias User = UsersApp.User

//     var isValid : Bool {
//         true
//     }
// }
