import { z } from "zod";

export const companyYamlSchema = z.object({
  name: z.string().min(1),
  slug: z
    .string()
    .min(1)
    .regex(/^[a-z0-9-]+$/, "slug must be lowercase alphanumeric with hyphens"),
  ats: z.enum([
    "greenhouse",
    "lever",
    "ashby",
    "workable",
    "smartrecruiters",
    "recruitee",
    "rippling",
    "bamboohr",
    "workday",
    "citadel",
    "citadel_securities",
    "tesla",
    "bytedance",
    "tiktok",
  ]),
  board_token: z.string().min(1),
  careers_url: z.string().url().optional(),
  website_url: z.string().url().optional(),
  active: z.boolean().optional().default(true),
});

export type CompanyYaml = z.infer<typeof companyYamlSchema>;

export const companiesFileSchema = z.object({
  companies: z.array(companyYamlSchema).min(1),
});

export const tier1CuratedSchema = z.object({
  slugs: z
    .array(
      z
        .string()
        .min(1)
        .regex(/^[a-z0-9-]+$/, "slug must be lowercase alphanumeric with hyphens"),
    )
    .min(1),
});

export type Tier1Curated = z.infer<typeof tier1CuratedSchema>;
