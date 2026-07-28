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

  app.middleware.use(app.sessions.middleware)

  guard let dbUrl = Environment.get("DATABASE_URL") else {
    fatalError("DATABASE_URL is not set in environment")
  }
  app.logger.debug("DATABASE_URL: \(dbUrl)")
  try app.databases.use(.mongo(connectionString: dbUrl), as: .mongo)

  let redisHostname = Environment.get("REDIS_HOSTNAME") ?? "localhost"
  let redisConfig = try RedisConfiguration(hostname: redisHostname)
  app.redis.configuration = redisConfig

  try migrations(app)

  app.sessions.use(.redis)
  app.caches.use(.fluent)

  try routes(app)

  try app.autoMigrate().wait()
}
