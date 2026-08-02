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

  guard let dbUrl = Environment.get("DATABASE_URL") else {
    fatalError("DATABASE_URL is not set in environment")
  }
  app.logger.debug("DATABASE_URL: \(dbUrl)")
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
