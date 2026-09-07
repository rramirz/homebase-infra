CREATE TABLE estate_beneficiaries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    name text NOT NULL CHECK (btrim(name) <> ''),
    relationship text NOT NULL DEFAULT '',
    dependent boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_trusts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    name text NOT NULL CHECK (btrim(name) <> ''),
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_trust_designations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    trust_id uuid NOT NULL,
    beneficiary_id uuid NOT NULL,
    share_percent numeric(7,6) NOT NULL CHECK (share_percent > 0 AND share_percent <= 1),
    tier text NOT NULL CHECK (tier IN ('primary', 'contingent')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    UNIQUE (trust_id, beneficiary_id, tier),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (trust_id, user_id, household_id) REFERENCES estate_trusts(id, user_id, household_id) ON DELETE CASCADE,
    FOREIGN KEY (beneficiary_id, user_id, household_id) REFERENCES estate_beneficiaries(id, user_id, household_id) ON DELETE RESTRICT
);

CREATE TABLE estate_assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    name text NOT NULL CHECK (btrim(name) <> ''),
    value numeric(20,2) NOT NULL CHECK (value >= 0),
    liquid boolean NOT NULL DEFAULT false,
    transfer_method text NOT NULL CHECK (transfer_method IN ('probate', 'wros', 'designation', 'trust')),
    trust_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (trust_id, user_id, household_id) REFERENCES estate_trusts(id, user_id, household_id) ON DELETE RESTRICT,
    CHECK ((transfer_method = 'trust' AND trust_id IS NOT NULL) OR (transfer_method <> 'trust' AND trust_id IS NULL))
);

CREATE TABLE estate_asset_owners (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    owner_name text NOT NULL CHECK (btrim(owner_name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    UNIQUE (asset_id, owner_name),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (asset_id, user_id, household_id) REFERENCES estate_assets(id, user_id, household_id) ON DELETE CASCADE
);

CREATE TABLE estate_asset_designations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    asset_id uuid NOT NULL,
    beneficiary_id uuid NOT NULL,
    share_percent numeric(7,6) NOT NULL CHECK (share_percent > 0 AND share_percent <= 1),
    tier text NOT NULL CHECK (tier IN ('primary', 'contingent')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    UNIQUE (asset_id, beneficiary_id, tier),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (asset_id, user_id, household_id) REFERENCES estate_assets(id, user_id, household_id) ON DELETE CASCADE,
    FOREIGN KEY (beneficiary_id, user_id, household_id) REFERENCES estate_beneficiaries(id, user_id, household_id) ON DELETE RESTRICT
);

CREATE TABLE estate_life_insurance (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    policy_name text NOT NULL CHECK (btrim(policy_name) <> ''),
    benefit_amount numeric(20,2) NOT NULL CHECK (benefit_amount >= 0),
    in_estate boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_insurance_designations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    insurance_id uuid NOT NULL,
    beneficiary_id uuid NOT NULL,
    share_percent numeric(7,6) NOT NULL CHECK (share_percent > 0 AND share_percent <= 1),
    tier text NOT NULL CHECK (tier IN ('primary', 'contingent')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    UNIQUE (insurance_id, beneficiary_id, tier),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (insurance_id, user_id, household_id) REFERENCES estate_life_insurance(id, user_id, household_id) ON DELETE CASCADE,
    FOREIGN KEY (beneficiary_id, user_id, household_id) REFERENCES estate_beneficiaries(id, user_id, household_id) ON DELETE RESTRICT
);

CREATE TABLE estate_legal_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    document_type text NOT NULL CHECK (btrim(document_type) <> ''),
    storage_location text NOT NULL CHECK (btrim(storage_location) <> ''),
    status text NOT NULL CHECK (status IN ('unknown', 'draft', 'signed', 'needs_review')),
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_contacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    role text NOT NULL CHECK (btrim(role) <> ''),
    name text NOT NULL CHECK (btrim(name) <> ''),
    contact_info text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_digital_assets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    service_name text NOT NULL CHECK (btrim(service_name) <> ''),
    account_identifier text NOT NULL DEFAULT '',
    access_instructions_location text NOT NULL CHECK (btrim(access_instructions_location) <> ''),
    notes text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_residuary_shares (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    beneficiary_id uuid NOT NULL,
    share_percent numeric(7,6) NOT NULL CHECK (share_percent > 0 AND share_percent <= 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    UNIQUE (household_id, beneficiary_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (beneficiary_id, user_id, household_id) REFERENCES estate_beneficiaries(id, user_id, household_id) ON DELETE RESTRICT
);

CREATE TABLE estate_scenarios (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    household_id uuid NOT NULL,
    name text NOT NULL CHECK (btrim(name) <> ''),
    decedent_name text NOT NULL CHECK (btrim(decedent_name) <> ''),
    as_of_year integer NOT NULL CHECK (as_of_year BETWEEN 1900 AND 3000),
    death_year integer NOT NULL CHECK (death_year BETWEEN as_of_year AND 3000),
    debt_amount numeric(20,2) NOT NULL DEFAULT 0 CHECK (debt_amount >= 0),
    funeral_cost numeric(20,2) NOT NULL DEFAULT 0 CHECK (funeral_cost >= 0),
    legal_cost numeric(20,2) NOT NULL DEFAULT 0 CHECK (legal_cost >= 0),
    pay_debts boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE
);

CREATE TABLE estate_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    household_id uuid NOT NULL,
    scenario_id uuid NOT NULL,
    input_snapshot jsonb NOT NULL,
    result_payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id, household_id),
    FOREIGN KEY (household_id, user_id) REFERENCES households(id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (scenario_id, user_id, household_id) REFERENCES estate_scenarios(id, user_id, household_id) ON DELETE CASCADE
);

CREATE INDEX estate_assets_scope_idx ON estate_assets (user_id, household_id);
CREATE INDEX estate_results_scenario_idx ON estate_results (user_id, household_id, scenario_id, created_at DESC);
