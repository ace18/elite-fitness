// plans.js — the custom-plan questionnaire and the recommendation engine that
// turns answers into a program.
//
// The curated programs used to live here as PREMADE_PLANS. They now come from
// GET /api/plans (the plan_templates table), so a template added by a
// migration shows up in the browser without touching this file.

export const normalizePlan = (p) => ({ ...p, week: 1 });

// recommend a program from the questionnaire answers
export function recommendPlan(a) {
  // Each entry keeps the English value *and* a display key. The English one
  // is what gets sent to /api/plans/generate — the AI prompt and the stored
  // program stay in one canonical language regardless of the UI locale.
  const byGoal = {
    muscle:   { name: 'Hypertrophy Split',     goal: 'Hypertrophy',    focus: 'Volume · 6–12 reps',        key: 'muscle' },
    strength: { name: '5×5 Strength',          goal: 'Max strength',   focus: 'Heavy compounds · 3–6 reps', key: 'strength' },
    lean:     { name: 'Lean & Conditioned',    goal: 'Fat loss',       focus: 'Supersets · short rest',     key: 'lean' },
    fit:      { name: 'Full-Body Foundations', goal: 'General fitness',focus: 'Balanced · full body',       key: 'fit' }
  };
  const base = byGoal[a.goal] || byGoal.fit;
  const weeks = a.level === 'beginner' ? 8 : a.level === 'advanced' ? 5 : 6;
  return {
    id: 'gen-' + a.goal,
    name: base.name,
    goal: base.goal,
    focus: base.focus,
    level: a.level === 'beginner' ? 'Beginner' : a.level === 'advanced' ? 'Advanced' : 'Intermediate',
    // Display keys — resolved through $t by the plan screen. Premade plans
    // come from the API without these and fall back to their DB strings.
    nameKey: `plan.rec.${base.key}Name`,
    goalKey: `plan.rec.${base.key}Goal`,
    focusKey: `plan.rec.${base.key}Focus`,
    levelKey: `plan.q.${a.level === 'beginner' ? 'beginner' : a.level === 'advanced' ? 'advanced' : 'intermediate'}`,
    week: 1,
    totalWeeks: weeks,
    daysPerWeek: a.days,
    sessionMin: a.length
  };
}

// The questionnaire carries i18n keys, not copy — the plan page resolves them
// through $t so switching language re-renders the quiz.
export const PLAN_QUESTIONS = [
  {
    key: 'goal', title: 'plan.q.goalTitle', sub: 'plan.q.goalSub', kind: 'cards',
    options: [
      { v: 'muscle',   glyph: '💪', title: 'plan.q.muscle',   desc: 'plan.q.muscleDesc' },
      { v: 'strength', glyph: '🏋', title: 'plan.q.strength', desc: 'plan.q.strengthDesc' },
      { v: 'lean',     glyph: '🔥', title: 'plan.q.lean',     desc: 'plan.q.leanDesc' },
      { v: 'fit',      glyph: '⚡', title: 'plan.q.fit',      desc: 'plan.q.fitDesc' }
    ]
  },
  {
    key: 'level', title: 'plan.q.levelTitle', sub: 'plan.q.levelSub', kind: 'cards',
    options: [
      { v: 'beginner',     glyph: '🌱', title: 'plan.q.beginner',     desc: 'plan.q.beginnerDesc' },
      { v: 'intermediate', glyph: '📈', title: 'plan.q.intermediate', desc: 'plan.q.intermediateDesc' },
      { v: 'advanced',     glyph: '🎯', title: 'plan.q.advanced',     desc: 'plan.q.advancedDesc' }
    ]
  },
  {
    key: 'days', title: 'plan.q.daysTitle', sub: 'plan.q.daysSub', kind: 'tiles',
    options: [{ v: 2, label: 'common.days' }, { v: 3, label: 'common.days' }, { v: 4, label: 'common.days' }, { v: 5, label: 'common.days' }, { v: 6, label: 'common.days' }], unit: 'common.days'
  },
  {
    key: 'length', title: 'plan.q.lengthTitle', sub: 'plan.q.lengthSub', kind: 'tiles',
    options: [{ v: 30, label: 'common.min' }, { v: 45, label: 'common.min' }, { v: 60, label: 'common.min' }, { v: 75, label: 'common.min' }], unit: 'common.min'
  }
];

export const BUILD_MSGS = [
  'plan.build.analyzing',
  'plan.build.selecting',
  'plan.build.balancing',
  'plan.build.progression'
];
