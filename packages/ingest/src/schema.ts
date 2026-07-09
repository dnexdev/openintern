import { z } from "zod";

export const companyYamlSchema = z.object({
  name: z.string().min(1),
  slug: z
    .string()
    .min(1)
    .regex(/^[a-z0-9-]+$/, "slug must be lowercase alphanumeric with hyphens"),
  ats: z.enum(["greenhouse", "lever", "ashby", "workable", "smartrecruiters"]),
  board_token: z.string().min(1),
  careers_url: z.string().url().optional(),
  website_url: z.string().url().optional(),
  active: z.boolean().optional().default(true),
});

export type CompanyYaml = z.infer<typeof companyYamlSchema>;

export const companiesFileSchema = z.object({
  companies: z.array(companyYamlSchema).min(1),
});
