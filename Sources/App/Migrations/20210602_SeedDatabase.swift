//
// 20210602_SeedDatabase.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Fluent
import Foundation
import UserModel
import Vapor

struct SeedDatabase: Migration {
  func prepare(on database: Database) -> EventLoopFuture<Void> {
    //        let passwordHash: String
    //        do {
    //            let password = [UInt8].random(count: 32).map {
    //                $0 % 32
    //            }.base64
    //            print("admin password: \(password)")
    //            passwordHash = try Bcrypt.hash(password)
    //        }
    //        catch {
    //            return database.eventLoop.future(error: error)
    //        }

    //        let user = User(name: "Admin", username: "admin", /*password: passwordHash, */ email: "admin@localhost.local")
    let user = User(name: "Admin", email: "paul@schifferers.net")

    // username: "paulyhedral", thirdPartyAuth: "github", thirdPartyAuthId: "paulyhedral",
    // TODO: create admin role and link to user
    return user.save(on: database)
    //                   .flatMap { user in
    //                       let profile = LoginProfile(thirdPartyAuth: "github", thirdPartyAuthId: "paulyhedral", username: "paulyhedral",  userId: try! user.requireID())
    //                       return profile.save(on: database)
    //                   }
  }

  func revert(on database: Database) -> EventLoopFuture<Void> {
    User.query(on: database)
      .filter(\.$name == "Admin")
      .delete()
  }
}
