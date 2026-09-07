-- Estate results are immutable run records. Snapshot the scenario fields the
-- report renders so later scenario edits or deletion cannot mutate results.

ALTER TABLE estate_results
    ADD COLUMN scenario_name text NOT NULL DEFAULT '',
    ADD COLUMN decedent_name text NOT NULL DEFAULT '',
    ADD COLUMN as_of_year integer NOT NULL DEFAULT 0,
    ADD COLUMN death_year integer NOT NULL DEFAULT 0;

UPDATE estate_results r
SET scenario_name = s.name,
    decedent_name = s.decedent_name,
    as_of_year = s.as_of_year,
    death_year = s.death_year
FROM estate_scenarios s
WHERE s.id = r.scenario_id;

-- Scenario deletion must not destroy results: scenario_id stays nullable and is
-- set to NULL on scenario delete. The composite scoped FK stays in place, so DB
-- scoping remains enforced: only scenario_id is nulled, and cross-household
-- references stay impossible at the database level.
ALTER TABLE estate_results DROP CONSTRAINT estate_results_scenario_id_user_id_household_id_fkey;
ALTER TABLE estate_results ALTER COLUMN scenario_id DROP NOT NULL;
ALTER TABLE estate_results ADD CONSTRAINT estate_results_scenario_id_user_id_household_id_fkey FOREIGN KEY (scenario_id, user_id, household_id) REFERENCES estate_scenarios(id, user_id, household_id) ON DELETE SET NULL (scenario_id);
