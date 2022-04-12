CREATE TABLE records (tran_id string not null, tran_type string not null, amount string not null, currency string not null, tran_date string not null, remarks2 string not null, remarks string not null, balance_currency string not null, posted_date string not null, tran_description string not null);
CREATE TABLE accounts (number string not null, name string not null, client_id string not null, client_secret string not null, username string not null, password string not null, partner_id string not null);
CREATE UNIQUE INDEX index_unique_tran_id_tran_type on records (tran_id, tran_type);
