// plans.js — curated programs, the custom-plan questionnaire, and the
// recommendation engine that turns answers into a program.

export const PREMADE_PLANS = [
  { id: 'ppl-hyp',     glyph: '💪', name: 'Push / Pull / Legs',     goal: 'Hypertrophy',     focus: 'Build size with high weekly volume',    level: 'Intermediate', daysPerWeek: 6, sessionMin: 60, totalWeeks: 8, tag: 'Most popular' },
  { id: 'str-5x5',     glyph: '🏋', name: '5×5 Strength',           goal: 'Max strength',    focus: 'Heavy compounds, simple progression',   level: 'Beginner',     daysPerWeek: 3, sessionMin: 45, totalWeeks: 12 },
  { id: 'upper-lower', glyph: '🔁', name: 'Upper / Lower',          goal: 'Strength + size', focus: 'Balanced split, four focused days',      level: 'Intermediate', daysPerWeek: 4, sessionMin: 55, totalWeeks: 8 },
  { id: 'lean-cond',   glyph: '🔥', name: 'Lean & Conditioned',     goal: 'Fat loss',        focus: 'Supersets and short rest to stay lean',  level: 'All levels',   daysPerWeek: 4, sessionMin: 40, totalWeeks: 6 },
  { id: 'full-body',   glyph: '🌱', name: 'Full-Body Foundations',  goal: 'General fitness', focus: 'Master the basics, train full body',     level: 'Beginner',     daysPerWeek: 3, sessionMin: 35, totalWeeks: 8 }
];

export const normalizePlan = (p) => ({ ...p, week: 1 });

// recommend a program from the questionnaire answers
export function recommendPlan(a) {
  const byGoal = {
    muscle:   { name: 'Hypertrophy Split',     goal: 'Hypertrophy',    focus: 'Volume · 6–12 reps' },
    strength: { name: '5×5 Strength',          goal: 'Max strength',   focus: 'Heavy compounds · 3–6 reps' },
    lean:     { name: 'Lean & Conditioned',    goal: 'Fat loss',       focus: 'Supersets · short rest' },
    fit:      { name: 'Full-Body Foundations', goal: 'General fitness',focus: 'Balanced · full body' }
  };
  const base = byGoal[a.goal] || byGoal.fit;
  const weeks = a.level === 'beginner' ? 8 : a.level === 'advanced' ? 5 : 6;
  return {
    id: 'gen-' + a.goal,
    name: base.name,
    goal: base.goal,
    focus: base.focus,
    level: a.level === 'beginner' ? 'Beginner' : a.level === 'advanced' ? 'Advanced' : 'Intermediate',
    week: 1,
    totalWeeks: weeks,
    daysPerWeek: a.days,
    sessionMin: a.length
  };
}

export const PLAN_QUESTIONS = [
  {
    key: 'goal', title: "What's your main goal?", sub: 'We tune volume and rep ranges to match.', kind: 'cards',
    options: [
      { v: 'muscle',   glyph: '💪', title: 'Build muscle', desc: 'Maximize size and volume' },
      { v: 'strength', glyph: '🏋', title: 'Get stronger', desc: 'Heavier lifts, lower reps' },
      { v: 'lean',     glyph: '🔥', title: 'Get lean',     desc: 'Burn fat, stay conditioned' },
      { v: 'fit',      glyph: '⚡', title: 'Stay fit',     desc: 'Balanced general fitness' }
    ]
  },
  {
    key: 'level', title: 'How experienced are you?', sub: 'Sets the starting weights and progression pace.', kind: 'cards',
    options: [
      { v: 'beginner',     glyph: '🌱', title: 'Beginner',     desc: 'New or returning after a break' },
      { v: 'intermediate', glyph: '📈', title: 'Intermediate', desc: '6 months – 2 years lifting' },
      { v: 'advanced',     glyph: '🎯', title: 'Advanced',     desc: '2+ years, consistent training' }
    ]
  },
  {
    key: 'days', title: 'Days per week?', sub: 'How many sessions can you commit to?', kind: 'tiles',
    options: [{ v: 2, label: 'days' }, { v: 3, label: 'days' }, { v: 4, label: 'days' }, { v: 5, label: 'days' }, { v: 6, label: 'days' }], unit: 'days'
  },
  {
    key: 'length', title: 'Time per session?', sub: 'We size each workout to fit.', kind: 'tiles',
    options: [{ v: 30, label: 'min' }, { v: 45, label: 'min' }, { v: 60, label: 'min' }, { v: 75, label: 'min' }], unit: 'min'
  }
];

export const BUILD_MSGS = [
  'Analyzing your goals…',
  'Selecting the right movements…',
  'Balancing weekly volume…',
  'Setting your progression…'
];
