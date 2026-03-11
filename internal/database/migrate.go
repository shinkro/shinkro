package database

const schema = `
CREATE TABLE anime 
(
	mal_id 		INTEGER PRIMARY KEY,
	title 		TEXT,
	en_title 	TEXT,
	anidb_id 	INTEGER,
	tvdb_id 	INTEGER,
	tmdb_id 	INTEGER,
	type 		TEXT,
	releaseDate TEXT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users
(
    id         INTEGER PRIMARY KEY,
    username   TEXT NOT NULL,
    password   TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (username)
);

CREATE TABLE api_key
(
    name       TEXT,
    key        TEXT PRIMARY KEY,
    scopes     TEXT []   DEFAULT '{}' NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE malauth 
(
	id 					INTEGER PRIMARY KEY,
	client_id 			TEXT,
	client_secret 		TEXT,
	access_token 		BLOB,
	token_iv 			BLOB,
	created_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE anime_update
(
	id 				INTEGER PRIMARY KEY,
	mal_id 			INTEGER NOT NULL,
	source_db 		TEXT NOT NULL,
	source_id 		INTEGER NOT NULL,
	episode_num 	INTEGER,
	season_num  	INTEGER,
	time_stamp  	TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	list_details 	TEXT,
	list_status  	TEXT,
	plex_id      	INTEGER
	    REFERENCES plex_payload
			ON DELETE CASCADE,
	status          TEXT,
	error_type      TEXT,
	error_message   TEXT
);

CREATE TABLE plex_payload
(
	id 							INTEGER PRIMARY KEY,
	rating              		REAL,
	event  						TEXT NOT NULL,
	source  					TEXT NOT NULL,
	account_title 				TEXT NOT NULL,
	guid_string                 TEXT,
	guids                       TEXT,
	grand_parent_key    		TEXT,
	grand_parent_title  		TEXT,
	metadata_index              INTEGER,
	library_section_title 		TEXT NOT NULL,
	parent_index 				INTEGER,
	title						TEXT,
	type						TEXT NOT NULL,
	time_stamp                  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	success                     BOOLEAN,
	error_type                  TEXT,
	error_msg                   TEXT
);

CREATE TABLE plex_settings
(
	id 							INTEGER PRIMARY KEY,
	host						TEXT NOT NULL,
	port						INTEGER,
	tls 						BOOLEAN,
	tls_skip_verify				BOOLEAN,
	token						BLOB,
	token_iv					BLOB,
	username					TEXT NOT NULL,
	anime_libraries             TEXT []   DEFAULT '{}' NOT NULL,
	plex_client_enabled			BOOLEAN DEFAULT false NOT NULL,
	client_id				    TEXT,
	time_stamp                  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE anilistauth 
(
	id 					INTEGER PRIMARY KEY,
	client_id 			TEXT,
	client_secret 		TEXT,
	redirect_url		TEXT,
	access_token 		BLOB,
	token_iv 			BLOB,
	oauth_state			TEXT,
	created_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE mapping_settings
(
	id						        INTEGER PRIMARY KEY,
	tvdb_enabled			BOOLEAN,
	tmdb_enabled		BOOLEAN,
	tvdb_path				TEXT,
	tmdb_path				TEXT,
	time_stamp             TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notification
(
	id         INTEGER PRIMARY KEY,
	name       TEXT,
	type       TEXT,
	enabled    BOOLEAN,
	events     TEXT []   DEFAULT '{}' NOT NULL,
	token      TEXT,
	api_key    TEXT,
	webhook    TEXT,
	title      TEXT,
	icon       TEXT,
	host       TEXT,
	username   TEXT,
	password   TEXT,
	channel    TEXT,
	rooms      TEXT,
	targets    TEXT,
	devices    TEXT,
	topic      TEXT,
	priority   INTEGER DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

`

var migrations = []string{
	"",
	`-- Add status columns to plex_payload
ALTER TABLE plex_payload ADD COLUMN success BOOLEAN;
ALTER TABLE plex_payload ADD COLUMN error_type TEXT;
ALTER TABLE plex_payload ADD COLUMN error_msg TEXT;

-- Migrate data from plex_status to plex_payload
UPDATE plex_payload 
SET success = (
	SELECT ps.success 
	FROM plex_status ps 
	WHERE ps.plex_id = plex_payload.id 
	LIMIT 1
),
error_msg = (
	SELECT ps.error_msg 
	FROM plex_status ps 
	WHERE ps.plex_id = plex_payload.id 
	LIMIT 1
)
WHERE EXISTS (
	SELECT 1 FROM plex_status ps WHERE ps.plex_id = plex_payload.id
);

-- Drop plex_status table
DROP TABLE IF EXISTS plex_status;
	-- Add status columns to anime_update
ALTER TABLE anime_update ADD COLUMN status TEXT;
ALTER TABLE anime_update ADD COLUMN error_type TEXT;
ALTER TABLE anime_update ADD COLUMN error_message TEXT;

-- Set status to SUCCESS for existing anime_update records
UPDATE anime_update SET status = 'SUCCESS' WHERE status IS NULL;`,
	`-- Add anilistauth table for AniList OAuth support
CREATE TABLE IF NOT EXISTS anilistauth 
(
	id 					INTEGER PRIMARY KEY,
	client_id 			TEXT,
	client_secret 		TEXT,
	redirect_url		TEXT,
	access_token 		BLOB,
	token_iv 			BLOB,
	oauth_state			TEXT,
	created_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at 			TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`,
}
