-- PostgreSQL schema (introspected from the live database). steam-ingress only writes to "Server";
-- it relies on the unique index "Server_Address_key" for INSERT ... ON CONFLICT ("Address").
CREATE TABLE "Server" (
	"ID" text PRIMARY KEY NOT NULL,
	"Address" text NOT NULL,
	"Country" text NOT NULL,
	"Latitute" double precision NOT NULL,
	"Longitude" double precision NOT NULL,
	"Status" integer DEFAULT 0 NOT NULL,
	"LastUpdated" timestamp(3)
);

CREATE TABLE "PlayersData" (
	"ServerID" text NOT NULL,
	"List" jsonb NOT NULL,
	"MaxLast24Hours" jsonb NOT NULL,
	"MaxLast7Days" jsonb NOT NULL,
	"MaxLast30Days" jsonb NOT NULL,
	"Timestamp" timestamp(3) NOT NULL
);

CREATE TABLE "ServerData" (
	"ServerID" text NOT NULL,
	"Hostname" text NOT NULL,
	"Map" text NOT NULL,
	"MaxPlayers" integer NOT NULL,
	"PlayersCount" integer NOT NULL,
	"BotsCount" integer NOT NULL,
	"Version" text NOT NULL,
	"Secure" boolean NOT NULL,
	"Tags" text NOT NULL
);

CREATE TABLE "Metrics" (
	"ID" serial PRIMARY KEY NOT NULL,
	"CheckedLast24Hours" jsonb NOT NULL,
	"CheckedLast7Days" jsonb NOT NULL,
	"CheckedLast30Days" jsonb NOT NULL,
	"PrefilterLast24Hours" jsonb NOT NULL,
	"PrefilterLast7Days" jsonb NOT NULL,
	"PrefilterLast30Days" jsonb NOT NULL,
	"PlayersLast24Hours" jsonb NOT NULL,
	"PlayersLast7Days" jsonb NOT NULL,
	"PlayersLast30Days" jsonb NOT NULL
);

CREATE TABLE "RateLimit" (
	"ID" text PRIMARY KEY NOT NULL,
	"IP" text NOT NULL,
	"Kind" text NOT NULL,
	"Count" integer DEFAULT 0 NOT NULL,
	"LastSeen" timestamp(3) NOT NULL,
	"WindowStart" timestamp(3) NOT NULL
);

CREATE TABLE "Tasks" (
	"ID" text PRIMARY KEY NOT NULL,
	"TaskKind" integer NOT NULL,
	"TaskData" jsonb NOT NULL,
	"TaskExecuted" integer DEFAULT 0 NOT NULL
);

ALTER TABLE "PlayersData" ADD CONSTRAINT "PlayersData_ServerID_fkey" FOREIGN KEY ("ServerID") REFERENCES "public"."Server"("ID") ON DELETE restrict ON UPDATE cascade;
ALTER TABLE "ServerData" ADD CONSTRAINT "ServerData_ServerID_fkey" FOREIGN KEY ("ServerID") REFERENCES "public"."Server"("ID") ON DELETE restrict ON UPDATE cascade;
CREATE UNIQUE INDEX "Server_Address_key" ON "Server" ("Address");
CREATE UNIQUE INDEX "PlayersData_ServerID_key" ON "PlayersData" ("ServerID");
CREATE UNIQUE INDEX "ServerData_ServerID_key" ON "ServerData" ("ServerID");
