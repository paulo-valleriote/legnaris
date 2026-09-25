// Runs once, on first volume initialization (empty data dir), before auth is
// enabled. Creates least-privileged users so services never connect as root.
//
//   legnaris_app       readWrite on `legnaris`         -> the Go API
//   legnaris_exporter  clusterMonitor on `admin`       -> mongodb-exporter
//   legnaris_view      read on `legnaris`              -> Mongo Express
//
// Passwords are read from the mongodb container environment (see
// docker-compose.yml).

const appDB = db.getSiblingDB("legnaris");

appDB.createUser({
  user: "legnaris_app",
  pwd: process.env.MONGO_APP_PASSWORD,
  roles: [{ role: "readWrite", db: "legnaris" }],
});

appDB.createUser({
  user: "legnaris_view",
  pwd: process.env.MONGO_VIEW_PASSWORD,
  roles: [{ role: "read", db: "legnaris" }],
});

db.getSiblingDB("admin").createUser({
  user: "legnaris_exporter",
  pwd: process.env.MONGO_EXPORTER_PASSWORD,
  roles: [{ role: "clusterMonitor", db: "admin" }],
});
