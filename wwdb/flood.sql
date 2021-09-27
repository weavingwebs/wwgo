CREATE TABLE flood (
  id BINARY(36) DEFAULT UUID() NOT NULL PRIMARY KEY,
  event VARCHAR(64) NOT NULL,
  identifier VARCHAR(128) NOT NULL,
  timestamp DATETIME DEFAULT NOW() NOT NULL,
  expiration DATETIME NOT NULL
);

CREATE INDEX flood_allow ON flood (event, identifier, timestamp);
CREATE INDEX flood_purge ON flood (expiration);
