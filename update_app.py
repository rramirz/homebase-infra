import os
import re

print("Updating internal/app/repository.go...")
repo_path = "internal/app/repository.go"
with open(repo_path, "r") as f:
    repo = f.read()

# Add Confidence to Resource
repo = repo.replace(
    "Name, Kind, Amount, StartYear, EndYear, Inflation, Extra string",
    "Name, Kind, Amount, StartYear, EndYear, Inflation, Extra, Confidence string"
)
# Add Confidence to Plan
repo = repo.replace(
    "AnnualReturn, Inflation string\n}",
    "AnnualReturn, Inflation, Confidence string\n}"
)

# Fix ListResources queries
repo = repo.replace(
    "SELECT id,name,COALESCE(kind,''),COALESCE(annual_amount,balance)::text,COALESCE(start_year,birth_year)::text,COALESCE(end_year,retirement_year)::text,COALESCE(inflation_rate,annual_rate)::text FROM",
    "SELECT id,name,COALESCE(kind,''),COALESCE(annual_amount,balance)::text,COALESCE(start_year,birth_year)::text,COALESCE(end_year,retirement_year)::text,COALESCE(inflation_rate,annual_rate)::text,confidence FROM"
)
repo = repo.replace(
    "SELECT id,name,'',balance::text,'','','' FROM liabilities",
    "SELECT id,name,'',balance::text,'','','',confidence FROM liabilities"
)
repo = repo.replace(
    "SELECT id,name,kind,balance::text,'','','' FROM accounts",
    "SELECT id,name,kind,balance::text,'','','',confidence FROM accounts"
)
repo = repo.replace(
    "SELECT id,name,'',birth_year::text,COALESCE(retirement_year::text,''),COALESCE(death_year::text,''),'' FROM people",
    "SELECT id,name,'',birth_year::text,COALESCE(retirement_year::text,''),COALESCE(death_year::text,''),'',confidence FROM people"
)
repo = repo.replace(
    "&x.StartYear, &x.EndYear, &x.Inflation)",
    "&x.StartYear, &x.EndYear, &x.Inflation, &x.Confidence)"
)

# Fix CreateResource queries
repo = repo.replace(
    "INSERT INTO people(user_id,household_id,name,birth_year,retirement_year,death_year)",
    "INSERT INTO people(user_id,household_id,name,birth_year,retirement_year,death_year,confidence)"
)
repo = repo.replace(
    "VALUES($1,$2,$3,$4::int,NULLIF($5,'')::int,NULLIF($6,'')::int)",
    "VALUES($1,$2,$3,$4::int,NULLIF($5,'')::int,NULLIF($6,'')::int,COALESCE(NULLIF($7,''),'unknown'))"
)
repo = repo.replace(
    "&x.StartYear, &x.EndYear)",
    "&x.StartYear, &x.EndYear, &x.Confidence)"
)

repo = repo.replace(
    "INSERT INTO accounts(user_id,household_id,name,kind,balance)",
    "INSERT INTO accounts(user_id,household_id,name,kind,balance,confidence)"
)
repo = repo.replace(
    "VALUES($1,$2,$3,$4,$5::numeric)",
    "VALUES($1,$2,$3,$4,$5::numeric,COALESCE(NULLIF($6,''),'unknown'))"
)
repo = repo.replace(
    "&x.Kind, &x.Amount)",
    "&x.Kind, &x.Amount, &x.Confidence)"
)

repo = repo.replace(
    "INSERT INTO liabilities(user_id,household_id,name,balance,annual_rate,annual_payment)",
    "INSERT INTO liabilities(user_id,household_id,name,balance,annual_rate,annual_payment,confidence)"
)
repo = repo.replace(
    "VALUES($1,$2,$3,$4::numeric,$5::numeric,$6::numeric)",
    "VALUES($1,$2,$3,$4::numeric,$5::numeric,$6::numeric,COALESCE(NULLIF($7,''),'unknown'))"
)
repo = repo.replace(
    "&x.Inflation, &x.EndYear)",
    "&x.Inflation, &x.EndYear, &x.Confidence)"
)

repo = repo.replace(
    "start_year,end_year,inflation_rate) VALUES",
    "start_year,end_year,inflation_rate,confidence) VALUES"
)
repo = repo.replace(
    "NULLIF($7,'')::int,$8::numeric)",
    "NULLIF($7,'')::int,$8::numeric,COALESCE(NULLIF($9,''),'unknown'))"
)
repo = repo.replace(
    "&x.EndYear, &x.Inflation)",
    "&x.EndYear, &x.Inflation, &x.Confidence)"
)

# UpdateResource
repo = repo.replace(
    "death_year=NULLIF($7,'')::int WHERE",
    "death_year=NULLIF($7,'')::int,confidence=COALESCE(NULLIF($8,''),'unknown') WHERE"
)
repo = repo.replace(
    "&x.StartYear, &x.EndYear}",
    "&x.StartYear, &x.EndYear, &x.Confidence}"
)

repo = repo.replace(
    "kind=$5,balance=$6::numeric WHERE",
    "kind=$5,balance=$6::numeric,confidence=COALESCE(NULLIF($7,''),'unknown') WHERE"
)
repo = repo.replace(
    "&x.Kind, &x.Amount}",
    "&x.Kind, &x.Amount, &x.Confidence}"
)

repo = repo.replace(
    "annual_rate=$6::numeric,annual_payment=$7::numeric WHERE",
    "annual_rate=$6::numeric,annual_payment=$7::numeric,confidence=COALESCE(NULLIF($8,''),'unknown') WHERE"
)
repo = repo.replace(
    "&x.Inflation, &x.EndYear}",
    "&x.Inflation, &x.EndYear, &x.Confidence}"
)

repo = repo.replace(
    "end_year=NULLIF($8,'')::int,inflation_rate=$9::numeric WHERE",
    "end_year=NULLIF($8,'')::int,inflation_rate=$9::numeric,confidence=COALESCE(NULLIF($10,''),'unknown') WHERE"
)
repo = repo.replace(
    "&x.EndYear, &x.Inflation}",
    "&x.EndYear, &x.Inflation, &x.Confidence}"
)


# Plan updates
repo = repo.replace(
    "annual_return::text,inflation_rate::text FROM",
    "annual_return::text,inflation_rate::text,confidence FROM"
)
repo = repo.replace(
    "&p.AnnualReturn, &p.Inflation)",
    "&p.AnnualReturn, &p.Inflation, &p.Confidence)"
)

repo = repo.replace(
    "annual_return,inflation_rate) VALUES",
    "annual_return,inflation_rate,confidence) VALUES"
)
repo = repo.replace(
    "$6::numeric,$7::numeric)",
    "$6::numeric,$7::numeric,COALESCE(NULLIF($8,''),'unknown'))"
)
repo = repo.replace(
    "p.AnnualReturn, p.Inflation)",
    "p.AnnualReturn, p.Inflation, p.Confidence)"
)

repo = repo.replace(
    "annual_return=$7::numeric,inflation_rate=$8::numeric WHERE",
    "annual_return=$7::numeric,inflation_rate=$8::numeric,confidence=COALESCE(NULLIF($9,''),'unknown') WHERE"
)

with open(repo_path, "w") as f:
    f.write(repo)
print("Updated repository.go")
