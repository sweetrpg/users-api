//
// configure.swift
// Copyright (c) 2021 Paul Schifferer.
//

import Common
import Fluent
import FluentMongoDriver
import Redis
import Vapor

public func configure(_ app: Application) throws {
  app.logger.logLevel = app.environment == .development ? .debug : .info

  app.middleware.use(SentryMiddleware())

  app.middleware.use(app.sessions.middleware)

  // DB_URI is a direct override (local dev, no-auth Mongo - see kubernetes/overlays/local's
  // secrets.yaml). Otherwise built from parts (DB_SCHEME/DB_HOST/DB_USER/DB_NAME/DB_OPTS from
  // the configmap, DB_PW from the Akeyless-sourced secret) rather than one opaque DATABASE_URL -
  // same pattern as admin-api/catalog-api's mongodb.go, since only the password is sensitive.
  let dbUrl: String
  if let dbUri = Environment.get("DB_URI") {
    dbUrl = dbUri
  } else {
    guard let dbScheme = Environment.get("DB_SCHEME"),
      let dbHost = Environment.get("DB_HOST"),
      let dbUser = Environment.get("DB_USER"),
      let dbPassword = Environment.get("DB_PW"),
      let dbName = Environment.get("DB_NAME")
    else {
      fatalError(
        "DB_URI, or DB_SCHEME/DB_HOST/DB_USER/DB_PW/DB_NAME, must be set in environment")
    }
    let encodedUser = dbUser.addingPercentEncoding(withAllowedCharacters: .urlUserAllowed) ?? dbUser
    let encodedPassword =
      dbPassword.addingPercentEncoding(withAllowedCharacters: .urlPasswordAllowed) ?? dbPassword
    var url = "\(dbScheme)://\(encodedUser):\(encodedPassword)@\(dbHost)/\(dbName)"
    if let dbOpts = Environment.get("DB_OPTS"), !dbOpts.isEmpty {
      url += "?\(dbOpts)"
    }
    dbUrl = url
  }
  try app.databases.use(.mongo(connectionString: dbUrl), as: .mongo)

  // DB index on the shared `redis.sweetrpg-support` instance - see sweetrpg/platform's
  // docs/frontend-conventions.md "Shared sweetrpg-support Redis instance" registry before
  // changing this index; it must match this service's row there.
  let redisHost = Environment.get("REDIS_HOST") ?? "localhost"
  let redisPort = Environment.get("REDIS_PORT").flatMap(Int.init) ?? 6379
  let redisDB = Environment.get("REDIS_DB").flatMap(Int.init)
  let redisConfig = try RedisConfiguration(hostname: redisHost, port: redisPort, database: redisDB)
  app.redis.configuration = redisConfig

  try migrations(app)

  app.sessions.use(.redis)
  app.caches.use(.fluent)

  try routes(app)

  try app.autoMigrate().wait()
}
